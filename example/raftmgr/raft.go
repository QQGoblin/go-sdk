package main

import (
	"context"
	"errors"
	"fmt"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	stats "go.etcd.io/etcd/server/v3/etcdserver/api/v2stats"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"
	"hash/crc32"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	DefaultHeartbeatTickMS = 100 * time.Millisecond
	DefaultHeartbeatTick   = 1
	DefaultElectionTick    = 10
	DefualtClusterID       = 0x1000
)

type RaftNode struct {
	id           uint64
	prefix       string
	raw          raft.Node
	config       *raft.Config
	tick         *time.Ticker
	storage      *raft.MemoryStorage
	wal          *wal.WAL
	waldir       string
	clusterPeers []raft.Peer
	peerURLs     map[uint64]string
	transport    *rafthttp.Transport
	stopc        chan struct{}
	httpstopc    chan struct{}
	logger       *zap.Logger
}

func (rc *RaftNode) Process(ctx context.Context, m raftpb.Message) error {
	return rc.raw.Step(ctx, m)
}
func (rc *RaftNode) IsIDRemoved(id uint64) bool  { return false }
func (rc *RaftNode) ReportUnreachable(id uint64) { rc.raw.ReportUnreachable(id) }
func (rc *RaftNode) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	rc.raw.ReportSnapshot(id, status)
}

func NewRaftNode(id uint64, peers map[uint64]string, waldir string, logger *zap.Logger) *RaftNode {

	raftStorage := raft.NewMemoryStorage()

	config := &raft.Config{
		Logger:          etcdserver.NewRaftLoggerZap(logger),
		PreVote:         true,
		ID:              id,
		ElectionTick:    DefaultElectionTick,
		HeartbeatTick:   DefaultHeartbeatTick,
		Storage:         raftStorage,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}

	clusterPeers := make([]raft.Peer, 0)
	for pId, _ := range peers {
		clusterPeers = append(clusterPeers, raft.Peer{
			ID: pId,
		})
	}

	rc := &RaftNode{
		id:           id,
		config:       config,
		tick:         time.NewTicker(DefaultHeartbeatTickMS),
		storage:      raftStorage,
		clusterPeers: clusterPeers,
		peerURLs:     peers,
		stopc:        make(chan struct{}),
		httpstopc:    make(chan struct{}),
		logger:       logger,
		waldir:       waldir,
	}

	return rc
}

func (rc *RaftNode) Start() {

	if rc.raw != nil {
		rc.raw.Stop()
	}

	oldWal := wal.Exist(rc.waldir)

	rc.wal = rc.openWAL()

	if oldWal {
		rc.raw = raft.RestartNode(rc.config)
	} else {
		rc.raw = raft.StartNode(rc.config, rc.clusterPeers)
	}

	if rc.transport != nil {
		rc.transport.Stop()
	}

	rc.transport = &rafthttp.Transport{
		//Logger:             rc.logger,
		DialTimeout: time.Second + DefaultElectionTick*DefaultHeartbeatTickMS,
		//DialRetryFrequency: 0,
		ID:          types.ID(rc.id),
		ClusterID:   DefualtClusterID,
		Raft:        rc,
		ServerStats: stats.NewServerStats("", fmt.Sprintf("%v", rc.id)),
		LeaderStats: stats.NewLeaderStats(rc.logger, fmt.Sprintf("%v", rc.id)),
		ErrorC:      make(chan error),
	}

	rc.transport.Start()

	for _, peer := range rc.clusterPeers {
		if peer.ID != rc.id {
			rc.transport.AddPeer(types.ID(peer.ID), []string{rc.peerURLs[peer.ID]})
		}
	}

	go rc.serveChannels()
	go rc.serveRaft()

	return
}

func (rc *RaftNode) Stop() {

	// 关闭 transport 服务
	if rc.transport != nil {
		rc.transport.Stop()
	}
	close(rc.httpstopc)

	// 停止 raft 服务
	if rc.raw != nil {
		rc.raw.Stop()
	}
	close(rc.stopc)
	return
}

func (rc *RaftNode) IsLeader() (bool, error) {

	if rc.raw == nil {
		return false, errors.New("raft node is not ready")
	}

	if s := rc.raw.Status(); &s == nil {
		return false, errors.New("not leader")
	} else {
		return s.ID == s.Lead, nil
	}
}

func (rc *RaftNode) serveChannels() {

	rc.tick.Reset(DefaultHeartbeatTickMS)
	defer rc.tick.Stop()

	defer rc.wal.Close()

	// event loop on raft state machine updates
	for {
		select {
		case <-rc.tick.C:
			rc.raw.Tick()

		// 转发请求到其他节点
		case rd := <-rc.raw.Ready():
			rc.wal.Save(rd.HardState, rd.Entries)
			rc.storage.Append(rd.Entries)
			rc.transport.Send(rd.Messages)
			rc.raw.Advance()
		case err := <-rc.transport.ErrorC:
			rc.logger.Error(err.Error())
			rc.Stop()
			break
		case <-rc.stopc:
			rc.Stop()
			break
		}
	}
	rc.logger.Info("stop success")
}

func (rc *RaftNode) serveRaft() {
	url, err := url.Parse(rc.peerURLs[rc.id])
	if err != nil {
		rc.logger.Fatal("failed parsing URL", zap.String("peer-url", rc.peerURLs[rc.id]))
	}

	ln, err := newStoppableListener(url.Host, rc.httpstopc)
	if err != nil {
		rc.logger.Fatal("failed to listen raft http", zap.String("host", url.String()))
	}

	err = (&http.Server{Handler: rc.transport.Handler()}).Serve(ln)
	select {
	case <-rc.httpstopc:
		rc.logger.Info("exit raft http server")
	default:
		rc.logger.Fatal("failed to serve raft http")
	}
}

func (rc *RaftNode) openWAL() *wal.WAL {

	rc.logger.Info("replaying WAL", zap.Uint64("id", rc.id))

	if !wal.Exist(rc.waldir) {
		if err := os.MkdirAll(rc.waldir, 0750); err != nil {
			rc.logger.Fatal(err.Error(), zap.String("wal-dir", rc.waldir))
		}

		w, err := wal.Create(zap.NewExample(), rc.waldir, nil)
		if err != nil {
			rc.logger.Fatal("create wal error")
		}
		w.Close()
	}

	w, err := wal.Open(zap.NewExample(), rc.waldir, walpb.Snapshot{})
	if err != nil {
		rc.logger.Fatal("error loading WAL")
	}

	_, st, ents, err := w.ReadAll()
	if err != nil {
		rc.logger.Fatal("failed to read WAL")
	}

	rc.storage.SetHardState(st)
	rc.storage.Append(ents)

	return w
}

func GetID(peerURL string) uint64 {
	pID := crc32.ChecksumIEEE([]byte(strings.TrimSpace(peerURL)))
	return uint64(pID)
}

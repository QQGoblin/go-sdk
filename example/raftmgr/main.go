package main

import (
	"context"
	"flag"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	peerURL         string
	clusterPeerURLs string
)

func init() {

	flag.StringVar(&peerURL, "url", "", "peer url")
	flag.StringVar(&clusterPeerURLs, "peers", "", "comma separated cluster peers")
}

var (
	zaplogger *zap.Logger
)

func main() {

	flag.Parse()

	var err error
	zaplogger, err = NewZaplogger()
	if err != nil {
		panic(err)
	}

	clusterPeers := make(map[uint64]string)

	for _, p := range strings.Split(clusterPeerURLs, ",") {
		clusterPeers[GetID(p)] = p
	}

	id := GetID(peerURL)
	clusterPeers[id] = peerURL

	zaplogger.Info("start raft node", zap.Uint64("id", id))
	zaplogger.Info("cluster node", zap.String("peers", fmt.Sprintf("%v", clusterPeers)))

	node := NewRaftNode(id, clusterPeers, zaplogger)

	node.Start()

	ctx, cancel := context.WithCancel(context.Background())

	go ReportClusterStatus(ctx, node)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-c
	cancel()
	node.Stop()

	return
}

func NewZaplogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	zap.NewExample()
	return config.Build()
}

func ReportClusterStatus(ctx context.Context, node *RaftNode) {
	t := time.NewTicker(3 * time.Second)
	for {
		select {
		case <-t.C:
			if isLeader, _ := node.IsLeader(); isLeader {
				// TODO: do something
				zaplogger.Info(">>> report cluster status: i am leader <<<")
			}
		case <-ctx.Done():
			t.Stop()
			return
		}
	}

}

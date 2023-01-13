package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type controller struct {
	id              string // 节点id信息，用于标识节点
	lockPrefix      string
	ttl             int
	endpoints       []string
	trustedCAFile   string
	certFile        string
	keyFile         string
	timeout         time.Duration
	refreshInterval time.Duration

	lock        *concurrency.Mutex
	lockSession *concurrency.Session
	cancel      context.CancelFunc
}

func NewController(id, lockPrefix, epStr string, ttl int, timeoutStr, refreshIntervalStr string, trustedCAFile, certFile, keyFile string) (*controller, error) {

	endpoints := strings.Split(epStr, ",")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, err
	}

	refreshInterval, err := time.ParseDuration(refreshIntervalStr)
	if err != nil {
		return nil, err
	}

	return &controller{
		id:              id,
		lockPrefix:      lockPrefix,
		endpoints:       endpoints,
		ttl:             ttl,
		trustedCAFile:   trustedCAFile,
		certFile:        certFile,
		keyFile:         keyFile,
		timeout:         timeout,
		refreshInterval: refreshInterval,
	}, nil
}

func (c *controller) start() {
	if c.cancel != nil {
		klog.Info("cancel election controller")
		c.cancel()
	}

	f := func(ctx context.Context) {

		if c.lockSession == nil {
			klog.Info("lockSession session is nil, try to create new session")
			if err := c.createSession(ctx); err != nil {
				klog.Errorf("try to create new session failed: %+v, retry on next loop", err)
				return
			}
		}

		isLeader := true
		ctxWithTimeout, _ := context.WithTimeout(ctx, c.timeout)
		if err := c.lock.TryLock(ctxWithTimeout); err != nil {
			if !errors.Is(err, concurrency.ErrLocked) {
				klog.Errorf("try connect etcd failed: %+v", err)
				c.cleanupSession()
				// TODO: 执行 lock 操作失败，可能 etcd 服务出现异常，需要执行清理操作
				return
			}
			isLeader = false
		}

		if isLeader {
			// TODO: i am leader!
			klog.Info("i am leader!")
		} else {
			// TODO: i am not leader!
			klog.Info("i am not leader!")
		}

		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	wait.JitterUntilWithContext(ctx, f, c.refreshInterval, -1, true)

	klog.Info("exit election loop")
}

func (c *controller) stop() {
	klog.Info("stop election controller")
	c.cleanupSession()
	c.cancel()
	// TODO: 退出竞争流程，执行清理操作
}

func (c *controller) cleanupSession() {
	if c.lock != nil {
		ctxWithTimeout, _ := context.WithTimeout(context.Background(), c.timeout)
		if err := c.lock.Unlock(ctxWithTimeout); err != nil {
			klog.Errorf("try to unlock from session failed: %+v", err)
		}
	}

	if c.lockSession != nil {
		if err := c.lockSession.Close(); err != nil {
			klog.Errorf("try to close session failed: %+v", err)
		}
	}
	c.lockSession.Client().Close()
	c.lockSession = nil
	c.lock = nil
}

func (c *controller) createSession(ctx context.Context) error {
	tlsInfo := &transport.TLSInfo{
		CertFile:      c.certFile,
		KeyFile:       c.keyFile,
		TrustedCAFile: c.trustedCAFile,
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return err
	}

	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:            c.endpoints,
		AutoSyncInterval:     60 * time.Second,
		DialTimeout:          20 * time.Second,
		DialKeepAliveTime:    3 * time.Second,
		DialKeepAliveTimeout: 5 * time.Second,
		TLS:                  tlsConfig,
	})
	if err != nil {
		return err
	}

	klog.Infof("create new session for etcd concurrency lib with TTL(%d)", c.ttl)
	session, err := concurrency.NewSession(etcdCli, concurrency.WithContext(ctx), concurrency.WithTTL(c.ttl))
	if err != nil {
		return err
	}

	klog.Infof("new etcd session lease id is %x", session.Lease())

	c.lock = concurrency.NewMutex(session, c.lockPrefix)
	c.lockSession = session

	//记录节点的信息
	if err := c.record(ctx, etcdCli, session.Lease()); err != nil {
		return err
	}

	return nil

}

func (c *controller) record(ctx context.Context, etcdCli *clientv3.Client, leaseId clientv3.LeaseID) error {

	key := fmt.Sprintf("%s_id/%s", c.lockPrefix, c.id)
	ctxWithTimeout, _ := context.WithTimeout(ctx, c.timeout)
	_, err := etcdCli.Put(ctxWithTimeout, key, fmt.Sprintf("%x", leaseId))
	if err != nil {
		klog.Errorf("record node id failed: %+v", err)
	}
	return err
}

var (
	endpoints       string
	trustedCAFile   string
	certFile        string
	keyFile         string
	ttl             int
	timeout         string
	refreshInterval string
)

func init() {
	flag.StringVar(&endpoints, "endpoints", "https://127.0.0.1:2379", "ETCD 连接地址")
	flag.StringVar(&trustedCAFile, "cacert", "", "ETCD 客户端 CA 证书")
	flag.StringVar(&certFile, "cert", "", "ETCD 客户端证书")
	flag.StringVar(&keyFile, "key", "", "ETCD 客户端 Key")
	flag.StringVar(&timeout, "timeout", "3s", "ETCD 连接超时时间")
	flag.IntVar(&ttl, "ttl", 15, "Leader 租约超时时间")
	flag.StringVar(&refreshInterval, "refresh-interval", "5s", "Leader 租约更新时间间隔")

}

func main() {
	flag.Parse()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	id, _ := os.Hostname()
	ctr, err := NewController(id, "/goblin", endpoints, ttl, timeout, refreshInterval, trustedCAFile, certFile, keyFile)
	if err != nil {
		klog.Fatalf("create controller failed: %+v", err)
	}
	// 运行主线程
	go ctr.start()
	select {
	case <-c:
		ctr.stop()
	}

}

package manager

import (
	"fmt"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"os"
	"strings"
	"time"
)

const (
	CACertFile           = "ca.crt"
	CertFile             = "cert.crt"
	KeyFile              = "cert.key"
	LockPrefix           = "/vipctrl"
	DialTimeout          = 3 * time.Second
	DialKeepAliveTime    = 3 * time.Second
	DialKeepAliveTimeout = 3 * time.Second
)

type config struct {
	Endpoints []string      `json:"endpoint"`
	Timeout   time.Duration `json:"timeout"`
	TTL       int           `json:"ttl"`
	Interval  time.Duration `json:"interval"`
}

type controller struct {
	id                string
	config            config
	lock              *concurrency.Mutex
	lockSession       *concurrency.Session
	lockSessionCancel context.CancelFunc

	arpingHelper  *arpingHelper
	macvlanHelper *macvlanHelper

	cancel context.CancelFunc
}

func NewController(epsStr, timeoutStr, intervalStr string, ttl int, address, mask, link, macvlanLink string) *controller {

	// 初始化 config 配置
	endpoints := strings.Split(epsStr, ",")
	if len(endpoints) <= 0 {
		klog.Fatalf("bad parameter: endpoints<%s>", epsStr)
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		klog.Fatalf("bad parameter: timeout<%s>", timeoutStr)
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		klog.Fatalf("bad parameter: interval<%s>", intervalStr)
	}

	config := config{
		Endpoints: endpoints,
		Timeout:   timeout,
		TTL:       ttl,
		Interval:  interval,
	}

	// 初始化 arping 工具
	arpingHelper := NewArpingHelper(address, link)

	// 初始化 macvlan 工具
	macvlanHelper, err := NewMacvlanHelper(address, mask, link, macvlanLink, "")
	if err != nil {
		klog.Fatalf("failed to initialize macvlan tool: %v", err)
	}

	// 节点 id
	id, _ := os.Hostname()

	c := &controller{
		id:            id,
		config:        config,
		arpingHelper:  arpingHelper,
		macvlanHelper: macvlanHelper,
	}

	// 初始化 etcd 连接
	if err := c.session(); err != nil {
		klog.Fatalf("failed to initialize etcd session: %v", err)
	}

	return c
}

// Start 启动选举服务
func (c *controller) Start() {

	klog.Info("start vipctrl controller")
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go wait.JitterUntilWithContext(ctx, c.election, c.config.Interval, -1, true)

}

// Stop 停止选举服务
func (c *controller) Stop() {

	klog.Info("stop vipctrl controller")

	c.cancel() // 停止 election 函数循环

	c.arpingHelper.Stop() // 停止节点 arping

	// 移除 macvlan 设备
	if err := c.macvlanHelper.Delete(); err != nil {
		klog.Fatalf("failed to remove the macvlan device: %v", err)
	}

	// 关闭 etcd session 连接
	c.closeSession()

}

// session 创建 etcd session
func (c *controller) session() error {

	tlsInfo := &transport.TLSInfo{
		CertFile:      CertFile,
		KeyFile:       KeyFile,
		TrustedCAFile: CACertFile,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return err
	}

	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints: c.config.Endpoints,
		//AutoSyncInterval:     60 * time.Second,
		DialTimeout:          DialTimeout,
		DialKeepAliveTime:    DialKeepAliveTime,
		DialKeepAliveTimeout: DialKeepAliveTimeout,
		TLS:                  tlsConfig,
	})

	klog.Infof("create new session for etcd concurrency lib with TTL(%d)", c.config.TTL)
	ctx, cancel := context.WithCancel(context.Background())
	session, err := concurrency.NewSession(etcdCli, concurrency.WithContext(ctx), concurrency.WithTTL(c.config.TTL))
	if err != nil {
		return err
	}

	klog.Infof("new session lease id is %x", session.Lease())

	c.lock = concurrency.NewMutex(session, LockPrefix)
	c.lockSession = session
	c.lockSessionCancel = cancel

	// 在 etcd 上记录 node id 和 session id 的映射关系
	key := fmt.Sprintf("%s_id/%s", LockPrefix, c.id)

	ctxWithTimeout, _ := context.WithTimeout(context.Background(), c.config.Timeout)

	if _, err := etcdCli.Put(ctxWithTimeout, key, fmt.Sprintf("%x", session.Lease())); err != nil {
		klog.Errorf("write node id failed: %+v", err)
	}

	return nil
}

func (c *controller) closeSession() {

	// 可能出现 Unlock 逻辑没有彻底关闭 keepalive goroute ，导致旧的 Lease 依然在保活，此时所有节点都无法获取 lock
	if c.lock != nil {
		ctxWithTimeout, _ := context.WithTimeout(context.Background(), c.config.Interval)
		if err := c.lock.Unlock(ctxWithTimeout); err != nil {
			klog.Fatalf("try to unlock failed: %+v", err)
		}
	}
	c.lock = nil

	// 释放 lock 后强制关闭 session
	if c.lockSessionCancel != nil {
		c.lockSessionCancel()
	}
	c.lockSession = nil

}

// election 选举逻辑
func (c *controller) election(ctx context.Context) {

	winner := true

	ctxWithTimeout, _ := context.WithTimeout(ctx, c.config.Timeout)
	// 获取锁
	if err := c.lock.TryLock(ctxWithTimeout); err != nil {

		// 尝试获取锁失败，删除 vip，直接退出
		if !errors.Is(err, concurrency.ErrLocked) {
			c.macvlanHelper.Delete()
			os.Exit(-1)
		}

		winner = false
	}

	if winner {
		// 设置 vip（接口要可重入）
		if err := c.macvlanHelper.Set(); err != nil {
			c.macvlanHelper.Delete()
			os.Exit(-1)
		}
		// 开始 arping 广播（接口要可重入）
		c.arpingHelper.Start()

	} else {
		// 删除 vip（接口要可重入）
		if err := c.macvlanHelper.Delete(); err != nil {
			klog.Fatalf("failed to remove the macvlan device: %v", err)
		}
		// 停止 arping 广播 （接口要可重入）
		c.arpingHelper.Stop()
	}
	return
}

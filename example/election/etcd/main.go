package main

import (
	"context"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	Endpoints = []string{"https://127.0.0.1:2379"}
	//DefaultTrustedCAFile = "tls/ca.crt"
	//DefaultCertFile      = "tls/apiserver-etcd-client.crt"
	//DefaultKeyFile       = "tls/apiserver-etcd-client.key"
	DefaultTrustedCAFile = "/etc/rcos_global/tls_key/rccpnuwa/ca/ca.pem"
	DefaultCertFile      = "/etc/kubernetes/pki/apiserver-etcd-client.crt"
	DefaultKeyFile       = "/etc/kubernetes/pki//apiserver-etcd-client.key"
	DefaultSyncInterval  = 60 * time.Second
	DefaultDialTimeout   = 20 * time.Second
	DefaultLockPrefix    = "/rccpnuwa/lock"
)

func main() {

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	ctx, canel := context.WithCancel(context.Background())

	lock, err := createLock(ctx, DefaultLockPrefix, 60)
	if err != nil {
		klog.Fatalf("create etcd lock failed,%+v", err)
	}
	// Lock 接口在获取到锁前会一直卡住 , TryLock 接口可以立即返回
	if err := lock.Lock(ctx); err != nil {
		klog.Fatalf("try to acquire lock failed,%+v", err)
	}
	klog.Info("i am new leader")

	//TODO: Leader 节点业务逻辑

	select {
	case <-c:
		// 收到系统退出时， etcd 的 lock 接口不会立即释放 lease 此处我们需要手工 revoke
		klog.Info("exit revoke lease")
		lock.Unlock(ctx)
		canel()
	}
}

func createLock(ctx context.Context, lockname string, ttl int) (*concurrency.Mutex, error) {
	tlsInfo := &transport.TLSInfo{
		CertFile:      DefaultCertFile,
		KeyFile:       DefaultKeyFile,
		TrustedCAFile: DefaultTrustedCAFile,
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, err
	}

	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:        Endpoints,
		AutoSyncInterval: DefaultSyncInterval,
		DialTimeout:      DefaultDialTimeout,
		TLS:              tlsConfig,
	})
	if err != nil {
		return nil, err
	}

	session, err := concurrency.NewSession(etcdCli, concurrency.WithContext(ctx), concurrency.WithTTL(ttl))
	if err != nil {
		return nil, err
	}
	return concurrency.NewMutex(session, lockname), nil

}

package main

import (
	"context"
	"flag"
	"github.com/QQGoblin/go-sdk/pkg/kubeutils"
	"github.com/QQGoblin/go-sdk/pkg/network"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	addr           string
	dev            string
	lease          string
	leaseNamespace string
	kubeconfig     string
	id             string
)

func init() {
	flag.StringVar(&addr, "addr", "", "service externalip")
	flag.StringVar(&dev, "dev", "", "bind network device from externalip")
	flag.StringVar(&lease, "lease", "", "the lease lock resource name")
	flag.StringVar(&leaseNamespace, "namespace", "default", "the lease lock resource namespace")
	flag.StringVar(&id, "id", uuid.New().String(), "the holder identity name")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")

}

type Speaker struct {
	address   string `json:"string"`
	device    string `json:"device"`
	stopSpeak chan int
	ticker    *time.Ticker
}

func main() {
	flag.Parse()

	if addr == "" || dev == "" || lease == "" {
		klog.Fatal("error input parameters")
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	speaker := Speaker{
		address:   addr,
		device:    dev,
		stopSpeak: make(chan int, 1),
	}

	kclient := kubeutils.GetClientSetOrDie(kubeconfig, "")
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lease,
			Namespace: leaseNamespace,
		},
		// 跟kubernetes集群关联起来
		Client: kclient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	go leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second, // 选举的任期，60s一个任期，如果在60s后没有renew，那么leader就会释放锁，重新选举
		RenewDeadline:   10 * time.Second, // renew的请求的超时时间
		RetryPeriod:     3 * time.Second,  // leader获取到锁后，renew leadership的间隔。非leader，抢锁成为leader的间隔（有1.2的jitter因子，详细看代码）

		// 回调函数的注册
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				speaker.Speak()
			},
			OnStoppedLeading: func() {
				speaker.Stop()
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					return
				}
				klog.Infof("new leader elected: %s", identity)
			},
		},
	})

	<-c
	cancel()
	speaker.Stop()

}

func (s *Speaker) Speak() {

	klog.Infof("start send gratuitous packets for address<%s>", s.address)

	// 运行主线程
	if s.ticker == nil {
		s.ticker = time.NewTicker(1100 * time.Millisecond)
	} else {
		s.ticker.Reset(1100 * time.Millisecond)
	}

	for {
		select {
		case <-s.ticker.C:
			network.ARPSendGratuitous(s.address, s.device, 1)
		case <-s.stopSpeak:
			s.ticker.Stop()
			goto END

		}
	}
END:
	klog.Info("stop send gratuitous packets")
}

func (s *Speaker) Stop() {
	s.ticker.Stop()
	s.stopSpeak <- 1
}

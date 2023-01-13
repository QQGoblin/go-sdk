package main

import (
	"context"
	"github.com/QQGoblin/go-sdk/pkg/docker"
	"github.com/QQGoblin/go-sdk/pkg/kubeutils"
	leaderelection "github.com/QQGoblin/go-sdk/pkg/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"time"
)

const (
	DefaultDockerSocket = "/var/run/docker.sock"
	DefaultTimeOut      = 3 * time.Second
)

func main() {

	kubeCli := kubeutils.GetClientSetOrDie("/etc/kubernetes/admin.conf", "127.0.0.1")
	id, err := os.Hostname()
	if err != nil {
		klog.Fatalf("Error get node hostname: %s", err.Error())
	}

	rlc := resourcelock.ResourceLockConfig{
		Identity:      id,
		EventRecorder: nil, // 暂时不添加
	}

	lock, err := resourcelock.New(resourcelock.LeasesResourceLock, "default", "hello", kubeCli.CoreV1(), kubeCli.CoordinationV1(), rlc)
	if err != nil {
		klog.Fatalf("error create lease resource lock: %s", err.Error())
	}

	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: time.Second * 15,
		RenewDeadline: time.Second * 10,
		RetryPeriod:   time.Second * 3,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Infof("I'm new leader, my name is %s", id)
			},
			OnStoppedLeading: func() {
				klog.Info("I'm no leader anymore")
			},
		},
		ReleaseOnCancel: true, // 当 context 退出时，释放 lock
	}

	scli, err := docker.NewSocketClient(DefaultDockerSocket, DefaultTimeOut)
	if err != nil {
		klog.Fatalf("error create docker client: %+v", err.Error())
	}

	LeaderHealthCheck := func(context.Context) error {
		_, err := scli.Info()
		return err
	}

	lewc, err := leaderelection.NewLeaderElectorWithConditions(id, leaderElectionConfig, LeaderHealthCheck)
	if err != nil {
		klog.Fatalf("error create leader elector: %+v", err.Error())
	}

	lewc.Startup()

	stopCh := make(chan os.Signal, 0)
	signal.Notify(stopCh, os.Interrupt, os.Kill)
	<-stopCh
	lewc.Stop()
	klog.Info("exit success.")

}

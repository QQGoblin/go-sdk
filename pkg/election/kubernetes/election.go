package kubernetes

import (
	"context"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"time"
)

type elector struct {
	kubecli        *kubernetes.Clientset
	id             string
	leaderElection *leaderelection.LeaderElector
}

const (
	DEFAULT_LOCK_NAMESPACE        = "default"
	DEFAULT_LOCK_LEASE_DURATION   = 15 * time.Second
	DEFAULT_LOCK_RENEW_DEADLINE   = 10 * time.Second
	DEFAULT_LOCK_RETRY_PERIOD     = 2 * time.Second
	DEFAULT_JOINING_ELECTOR_AGAIN = 30 * time.Second
)

// COPY from vendor/k8s.io/client-go/tools/leaderelection/leaderelection.go:165
type CallBack struct {
	// OnStartedLeading is called when a LeaderElector client starts leading
	OnStartedLeading func(context.Context)
	// OnStoppedLeading is called when a LeaderElector client stops leading
	OnStoppedLeading func()
	// OnNewLeader is called when the client observes a leader that is
	// not the previously observed leader. This includes the first observed
	// leader when the client starts.
	OnNewLeader func(identity string)
}

func NewElector(kubecli *kubernetes.Clientset, id string, lockname string, callback CallBack) (*elector, error) {

	rlc := resourcelock.ResourceLockConfig{
		Identity:      id,
		EventRecorder: nil, // 暂时不添加
	}

	lock, err := resourcelock.New(resourcelock.LeasesResourceLock, DEFAULT_LOCK_NAMESPACE, lockname, kubecli.CoreV1(), kubecli.CoordinationV1(), rlc)
	if err != nil {
		klog.Errorf("error create lease resource lock: %s", err.Error())
		return nil, err
	}

	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: DEFAULT_LOCK_LEASE_DURATION,
		RenewDeadline: DEFAULT_LOCK_RENEW_DEADLINE,
		RetryPeriod:   DEFAULT_LOCK_RETRY_PERIOD,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: callback.OnStartedLeading,
			OnStoppedLeading: callback.OnStoppedLeading,
			OnNewLeader:      callback.OnNewLeader,
		},
		ReleaseOnCancel: true, // 当 context 退出时，释放 lock
	}
	leaderElection, err := leaderelection.NewLeaderElector(leaderElectionConfig)
	if err != nil {
		klog.Errorf("error create leader elector: %s", err.Error())
		return nil, err
	}

	return &elector{
		leaderElection: leaderElection,
		kubecli:        kubecli,
		id:             id,
	}, nil
}

func (e *elector) Startup() {

	go wait.Forever(
		func() {
			klog.Infof("start leader election, id<%s>", e.id)
			// 通过 context.Background() 保证选举进程不会退出，即使 leader 因为 renew 失败导致 leaderElection.Run() 退出，仍然能够重新加入选举
			e.leaderElection.Run(context.Background())
			klog.Warningf("exit leader election, id<%s>", e.id)

		}, DEFAULT_JOINING_ELECTOR_AGAIN)

}

func (e *elector) IsLeader() bool {
	return e.leaderElection.IsLeader()
}

func (e *elector) GetLeader() string {
	return e.leaderElection.GetLeader()
}

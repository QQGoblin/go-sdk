package kubernetes

import (
	"context"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

type elector struct {
	kubecli        *kubernetes.Clientset
	id             string
	leaderElection *leaderelection.LeaderElector
	mutex          sync.Mutex
	cancel         context.CancelFunc
	active         bool
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
		cancel:         nil,
		active:         false,
	}, nil
}

func (e *elector) Startup() {

	klog.Info("start election program")

	if e.active {
		return
	}

	// 创建 context
	e.mutex.Lock()
	var ctx context.Context
	ctx, e.cancel = context.WithCancel(context.Background())
	e.active = true
	e.mutex.Unlock()

	// 支持在不影响主线程的情况下，启动或者停止选举任务
	go wait.UntilWithContext(ctx,
		func(ctx context.Context) {
			e.leaderElection.Run(ctx)
			klog.Warning("election program exit")
		}, DEFAULT_JOINING_ELECTOR_AGAIN)
}

func (e *elector) Stop() {
	klog.Info("stop election program")
	// 加锁
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if e.cancel != nil {
		e.active = false // 关闭 election 携程
		e.cancel()
		e.cancel = nil
	}
}

func (e *elector) IsLeader() bool {
	return e.leaderElection.IsLeader()
}

func (e *elector) GetLeader() string {
	return e.leaderElection.GetLeader()
}

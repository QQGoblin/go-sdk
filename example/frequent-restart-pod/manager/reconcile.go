package manager

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type reconciler struct {
	maxRestart int
	threshold  time.Duration
	cache      cache.Cache
	status     map[string]*record
}

func NewReconciler(maxRestart int, threshold time.Duration, cache cache.Cache) reconcile.Reconciler {

	return &reconciler{maxRestart: maxRestart, threshold: threshold, cache: cache,
		status: make(map[string]*record),
	}
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {

	var currentPod = &v1.Pod{}
	if err := r.cache.Get(ctx, request.NamespacedName, currentPod); err != nil {
		klog.Errorf("%s is not found in cache: %v", request.String(), err)
		return reconcile.Result{}, err
	}

	q, isExist := r.status[currentPod.Name]
	if !isExist {
		histroy := NewRecord(r.maxRestart)
		histroy.SetFirstSeen(currentPod)
		r.status[currentPod.Name] = histroy
		return reconcile.Result{}, nil
	}

	if isNeedUpdate(currentPod, q) {
		q.Push(currentPod)
		klog.Infof("Pod<%s> CrashLoopBackOff %d", currentPod.Name, q.restartCount)
	}

	return reconcile.Result{}, nil
}

func isNeedUpdate(newPod *v1.Pod, histroy *record) bool {

	newCStates := convertContainerStateMap(newPod)

	if histroy.Size() == 0 {
		for _, newCState := range newCStates {
			// 判断当前是否有 container 处于退出状态
			if isTerminated(newCState) {
				return true
			}
		}

		// 配置 livenessProbe 时，容器可能被直接重启，此时从观测者看 State 不经过 Terminated 和 Waiting 状态，而一直处于 Running 状态。
		// 此时比较 containerID 是否发生变化，来判断容器是否发生了重启。
		firstSeenCStates := convertContainerStateMap(histroy.firstSeen)
		for name, newCState := range newCStates {
			firstSeenCState := firstSeenCStates[name]
			if firstSeenCState.ContainerID != newCState.ContainerID {
				return true
			}
		}
		return false
	}

	// 判断 containerID 相比上一次记录是否发生变化
	lastCStates := convertContainerStateMap(histroy.Last())
	for name, newCState := range newCStates {
		lastCState := lastCStates[name]
		if lastCState.ContainerID != newCState.ContainerID {
			return true
		}
	}
	return false
}

func convertContainerStateMap(p *v1.Pod) map[string]v1.ContainerStatus {

	m := make(map[string]v1.ContainerStatus)
	if p == nil || p.Status.ContainerStatuses == nil {
		return m
	}
	for _, c := range p.Status.ContainerStatuses {
		m[c.Name] = c
	}
	return m
}

func isTerminated(s v1.ContainerStatus) bool {

	// State.Terminated 非空，表示容器退出
	if s.State.Terminated != nil {
		return true
	}

	// s.State.Waiting 非空，表示容器等待启动，原因可能是初次启动，或者故障重启
	if s.State.Waiting != nil {
		return s.State.Waiting.Reason == "CrashLoopBackOff"
	}

	return false
}

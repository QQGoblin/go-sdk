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
	status     map[string]*podQueue
}

func NewReconciler(maxRestart int, threshold time.Duration, cache cache.Cache) reconcile.Reconciler {

	return &reconciler{maxRestart: maxRestart, threshold: threshold, cache: cache,
		status: make(map[string]*podQueue),
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
		r.status[currentPod.Name] = NewPodQueue(r.maxRestart)
		return reconcile.Result{}, nil
	}

	lastPod := q.Last()

	if isNeedUpdate(currentPod, lastPod) {
		q.Push(currentPod)
		klog.Infof("Pod<%s> CrashLoopBackOff %d", currentPod.Name, q.count)
	}

	return reconcile.Result{}, nil
}

func isNeedUpdate(newPod, oldPod *v1.Pod) bool {

	newCStates := convertContainerStateMap(newPod)

	if oldPod == nil {
		for _, newCState := range newCStates {
			if isTerminated(newCState) {
				return true
			}
		}
		return false
	}

	oldCStates := convertContainerStateMap(oldPod)

	for name, newCState := range newCStates {
		oldCState := oldCStates[name]

		// 新旧容器的 ID 发生变化，并且新的容器处于退出状态。表示容器刚刚发生了退出
		if oldCState.ContainerID != newCState.ContainerID && isTerminated(newCState) {
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

	// 判断 container 是否处于终止状态
	if s.State.Terminated != nil {
		return true
	}

	if s.State.Waiting != nil {
		return s.State.Waiting.Reason == "CrashLoopBackOff"
	}

	return false
}

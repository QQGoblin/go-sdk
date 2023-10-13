package manager

import (
	"context"
	"encoding/json"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

const (
	SPPodRestartCount appv1.ReplicaSetConditionType = "SPPodRestartCount"
)

type reconciler struct {
	maxRestart int
	threshold  time.Duration
	cache      cache.Cache
	status     map[string]*record
	cli        *kubernetes.Clientset
}

func NewReconciler(maxRestart int, threshold time.Duration, cache cache.Cache, cli *kubernetes.Clientset) reconcile.Reconciler {

	return &reconciler{maxRestart: maxRestart, threshold: threshold, cache: cache,
		cli:    cli,
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
		if err := r.refreshOwner(ctx, currentPod, q.restartCount); err != nil {
			klog.Errorf("Patch SPPodRestartCount Condition failed: %v", err)
		}

	}

	return reconcile.Result{}, nil
}

func (r *reconciler) refreshOwner(ctx context.Context, p *v1.Pod, restartCount int) error {

	var rsOwnerReference *metav1.OwnerReference
	for _, o := range p.GetOwnerReferences() {
		// 只处理 ReplicaSet 管理的 pod
		if o.Kind == "ReplicaSet" {
			rsOwnerReference = &o
			break
		}
	}

	// POD 上游没有 controller 直接退出
	if rsOwnerReference == nil {
		return nil
	}

	var owner = &appv1.ReplicaSet{}
	if err := r.cache.Get(ctx, types.NamespacedName{
		Name:      rsOwnerReference.Name,
		Namespace: p.Namespace,
	}, owner); err != nil {
		klog.Errorf("%s is not found in cache: %v", owner, err)
		return err
	}

	statusConditions := make([]appv1.ReplicaSetCondition, 0)
	podRestartCond := make(map[string]int)
	for _, c := range owner.Status.Conditions {
		if c.Type == SPPodRestartCount && c.Message != "" {
			if err := json.Unmarshal([]byte(c.Message), &podRestartCond); err != nil {
				klog.Errorf("%s with error message %s", SPPodRestartCount, c.Message)
				continue
			}
		}
		statusConditions = append(statusConditions, *c.DeepCopy())
	}

	if oldCount, isOK := podRestartCond[p.Spec.NodeName]; isOK {
		podRestartCond[p.Spec.NodeName] = oldCount + 1
	} else {
		podRestartCond[p.Spec.NodeName] = restartCount
	}

	condMessage, _ := json.Marshal(podRestartCond)

	statusConditions = append(statusConditions, appv1.ReplicaSetCondition{
		Type:               SPPodRestartCount,
		Status:             v1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             string(SPPodRestartCount),
		Message:            string(condMessage),
	})
	patch := appv1.ReplicaSet{
		Status: *owner.Status.DeepCopy(),
	}
	patch.Status.Conditions = statusConditions
	patchByte, _ := json.Marshal(patch)
	if _, err := r.cli.AppsV1().ReplicaSets(p.Namespace).Patch(
		context.Background(), owner.Name, types.StrategicMergePatchType, patchByte, metav1.PatchOptions{}, "status",
	); err != nil {
		return err
	}

	return nil
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

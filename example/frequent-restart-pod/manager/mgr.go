package manager

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)

var scheme = runtime.NewScheme()

func RunOrDie(ctx context.Context, kubeconfig, namespace string, maxRestart int, threshold time.Duration) {

	if err := v1.AddToScheme(scheme); err != nil {
		klog.Fatalf("run controller manager failed: %v", err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatalf("build config from flags failed: %v", err)
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		NewCache: cache.BuilderWithOptions(
			cache.Options{
				Scheme:    scheme,
				Namespace: namespace,
			},
		),
	})
	if err != nil {
		klog.Fatalf("run controller manager failed: %v", err)
	}

	if err = ctrl.NewControllerManagedBy(mgr).For(&v1.Pod{}).
		WithEventFilter(ignoreUpdateAndGenericPredicate()). // 过滤事件类型
		Complete(NewReconciler(maxRestart, threshold, mgr.GetCache())); err != nil {
		klog.Fatalf("run controller manager failed: %v", err)
	}

	if err = mgr.Start(ctx); err != nil {
		klog.Fatalf("run controller manager failed: %v", err)
	}
}

// 过滤 update 和 generic 类型的事件
func ignoreUpdateAndGenericPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// 忽略 node 除了 labels 以外的变动
			if e.ObjectOld == nil {
				klog.Error(nil, "Update event has no old object to update", "event", e)
				return false
			}
			if e.ObjectNew == nil {
				klog.Error(nil, "Update event has no new object for update", "event", e)
				return false
			}
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
	}
}

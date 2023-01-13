package main

import (
	"github.com/QQGoblin/go-sdk/pkg/kubeutils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/sample-controller/pkg/signals"
	"time"
)

func main() {

	kubeconfig := "./kubeconfig"
	master := ""

	config := kubeutils.GetConfigOrDie(kubeconfig, master)

	dyClient := dynamic.NewForConfigOrDie(config)
	dyInforFactory := dynamicinformer.NewDynamicSharedInformerFactory(dyClient, time.Second*30)

	podGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	podInformer := dyInforFactory.ForResource(podGVR)
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: printUnstructedPod,
		UpdateFunc: func(oldObj, newObj interface{}) {
			// Obj is *unstructured.Unstructured
			oldPod := oldObj.(*unstructured.Unstructured)
			newPod := newObj.(*unstructured.Unstructured)
			if oldPod.GetResourceVersion() == newPod.GetResourceVersion() {
				return
			}
			printUnstructedPod(newPod)
		},
		DeleteFunc: printUnstructedPod,
	})

	stopCh := signals.SetupSignalHandler()

	dyInforFactory.Start(stopCh)
	dyInforFactory.WaitForCacheSync(stopCh)

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")
}

func printUnstructedPod(obj interface{}) {
	pod := obj.(*unstructured.Unstructured)
	klog.Info(pod.GetName())
}

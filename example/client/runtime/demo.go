package main

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {

	kubeconfig := "./kubeconfig"
	master := ""

	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	crS := runtime.NewScheme()
	cliOpt := runtimeclient.Options{
		Scheme: crS,
	}

	// 添加自定义对象的访问
	// samplev1alpha1.AddToScheme()

	runtimeCli, err := runtimeclient.New(config, cliOpt)
	if err != nil {
		klog.Fatalf("Error building runtime client: %s", err.Error())
	}

	pods := &corev1.PodList{}
	if err := runtimeCli.List(context.Background(), pods, runtimeclient.InNamespace("rccp")); err != nil {
		klog.Fatalf("Error get pod list: %s", err.Error())
	}

	for _, pod := range pods.Items {
		klog.Info(pod.Name)
	}

}

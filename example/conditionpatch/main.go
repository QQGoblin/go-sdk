package main

import (
	"context"
	"encoding/json"
	"flag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	podname        string
	namespace      string
	conditionType  string
	conditionReady bool
	kubeconfig     string
)

func init() {
	flag.StringVar(&podname, "podname", "", "需要更新容器")
	flag.StringVar(&namespace, "namespace", "default", "需要更新容器所处的命名空间")
	flag.StringVar(&conditionType, "type", "", "需要更新的 conditionType")
	flag.BoolVar(&conditionReady, "ready", false, "condition 是否就绪")
	flag.StringVar(&kubeconfig, "kubeconfig", "/etc/kubernetes/admin.conf", "")
}

func main() {

	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatal("load kubeconfig failed")
	}
	kubeCli, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal("create kubernetes client failed")
	}

	podCondition := corev1.PodCondition{
		Type:               corev1.PodConditionType(conditionType),
		LastTransitionTime: metav1.Now(),
	}
	if conditionReady {
		podCondition.Status = corev1.ConditionTrue
	} else {
		podCondition.Status = corev1.ConditionFalse
	}

	patch := corev1.Pod{Status: corev1.PodStatus{Conditions: []corev1.PodCondition{podCondition}}}
	patchByte, _ := json.Marshal(patch)
	_, err = kubeCli.CoreV1().Pods(namespace).Patch(context.Background(), podname, types.StrategicMergePatchType, patchByte, metav1.PatchOptions{}, "status")
	if err != nil {
		klog.Fatalf("patch failed: %v", err)
	}

	return
}

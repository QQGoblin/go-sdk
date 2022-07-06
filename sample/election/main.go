package main

import (
	"context"
	kubeElection "github.com/QQGoblin/go-sdk/pkg/election/kubernetes"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
)

func main() {

	kubeconfig := "/etc/kubernetes/admin.conf"
	if len(os.Args) > 1 {
		kubeconfig = os.Args[1]
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	kubeCli, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Error create kubernetes clientset: %s", err.Error())
	}

	id, err := os.Hostname()
	if err != nil {
		klog.Fatalf("Error get node hostname: %s", err.Error())
	}

	callBack := kubeElection.CallBack{
		OnStartedLeading: func(c context.Context) {
			klog.Info("become leader, renew lease.")
		},
		OnStoppedLeading: func() {
			klog.Info("no longer the leader, staying inactive.")
		},
		OnNewLeader: func(current_id string) {
			if current_id == id {
				klog.Info("still the leader!")
				return
			}
			klog.Infof("new leader is %s", current_id)
		},
	}

	elector, err := kubeElection.NewElector(kubeCli, id, "goblin", callBack)

	elector.Startup()

	stopCh := make(chan os.Signal, 0)
	signal.Notify(stopCh, os.Interrupt, os.Kill)
	<-stopCh
	klog.Info("exit success.")

}

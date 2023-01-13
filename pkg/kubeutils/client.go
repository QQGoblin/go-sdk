package kubeutils

import (
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"os"
)

func GetConfigOrDie(kubeconfig, kubeApiserver string) *restclient.Config {

	var (
		restConfig *restclient.Config
		err        error
	)
	if len(kubeconfig) == 0 {
		klog.Info("create kubeconfig in cluster")
		restConfig, err = clientcmd.BuildConfigFromFlags("", "")
	} else {
		klog.Infof("create kubeconfig from kubeconfig file %s, master url is %s", kubeconfig, kubeApiserver)
		restConfig, err = clientcmd.BuildConfigFromFlags(kubeApiserver, kubeconfig)
	}
	if err != nil {
		klog.Error(err, "unable to get kubeconfig")
		os.Exit(1)
	}
	return restConfig
}

func GetClientSetOrDie(kubeconfig, kubeApiserver string) *kubernetes.Clientset {
	config := GetConfigOrDie(kubeconfig, kubeApiserver)
	kubeCli, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Error create kubernetes clientset: %s", err.Error())
	}
	return kubeCli
}

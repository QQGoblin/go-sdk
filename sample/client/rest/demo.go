package main

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"time"
)

func main() {

	kubeconfig := "./kubeconfig"
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	gv := corev1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/api"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	coreV1Cli, err := rest.RESTClientFor(config)
	if err != nil {
		klog.Fatalf("Error building rest client: %s", err.Error())
	}

	pods := &corev1.PodList{}
	opts := &metav1.ListOptions{}

	if err := coreV1Cli.Get().
		Resource("pods").
		Namespace("rccp").
		VersionedParams(opts, scheme.ParameterCodec).
		Timeout(time.Second * 60).
		Do(context.Background()).
		Into(pods); err != nil {
		klog.Fatalf("Error list pod: %s", err.Error())
	}

	for _, pod := range pods.Items {
		klog.Info(pod.Name)
	}

}

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/QQGoblin/go-sdk/pkg/kubeutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	podName      string
	podNamespace string
	kubeconfig   string
)

func init() {
	flag.StringVar(&podName, "pod", "", "pod name will be evict other pod")
	flag.StringVar(&podNamespace, "namespace", "default", "namespace")
	flag.StringVar(&kubeconfig, "kubeconfig", "/etc/kubernetes/admin.conf", "kubeconfig")
}

func GetPodOwners(dyClient dynamic.Interface, clientset *kubernetes.Clientset, name, namespace string) ([]*unstructured.Unstructured, error) {
	podGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	pod, err := dyClient.Resource(podGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if len(pod.GetOwnerReferences()) == 0 {
		return nil, fmt.Errorf("pod %s is no OwnerReferences", name)
	}

	owners := make([]*unstructured.Unstructured, 0)
	for _, ownerRef := range pod.GetOwnerReferences() {

		apiResource, err2 := clientset.Discovery().ServerResourcesForGroupVersion(ownerRef.APIVersion)
		if err2 != nil {
			return nil, err2
		}

		gv, _ := schema.ParseGroupVersion(apiResource.GroupVersion)

		var gvr *schema.GroupVersionResource
		var isNamespaced bool
		for _, r := range apiResource.APIResources {

			if r.Kind == ownerRef.Kind {
				gvr = &schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: r.Name,
				}
				isNamespaced = r.Namespaced
				break
			}
		}

		if gvr == nil {
			return nil, fmt.Errorf("owner gvr is not found")
		}
		var (
			err3 error
			obj  *unstructured.Unstructured
		)
		if isNamespaced {
			obj, err3 = dyClient.Resource(*gvr).Namespace(namespace).Get(context.Background(), ownerRef.Name, metav1.GetOptions{})
		} else {
			obj, err3 = dyClient.Resource(*gvr).Get(context.Background(), ownerRef.Name, metav1.GetOptions{})
		}

		if err3 != nil {
			return nil, err3
		}
		owners = append(owners, obj)
	}
	return owners, nil
}

func main() {
	flag.Parse()

	config := kubeutils.GetConfigOrDie(kubeconfig, "")

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Error create kubernetes clientset: %s", err.Error())
	}

	dyClient := dynamic.NewForConfigOrDie(config)

	owners, err := GetPodOwners(dyClient, client, podName, podNamespace)
	if err != nil {
		klog.Fatalf("Error get pod owner: %s", err.Error())
	}

	for _, o := range owners {
		klog.Infof("owner name %s, type %s/%s", o.GetName(), o.GetAPIVersion(), o.GetKind())
	}

}

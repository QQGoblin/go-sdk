package kubeutils

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TODO: filter.go 中的函数主要用于快速筛选出一组资源中的某些属性，如：所有 node 节点的 InteralIP，实现这些功能似乎使用 DynamicClient 操作 Unstructured 对象比较好

func FilterNodesWithInteralIP(kubecli *kubernetes.Clientset) (map[string]string, error) {
	return FilterNodes(kubecli, func(node *corev1.Node) string {
		for _, nodeAddress := range node.Status.Addresses {
			if nodeAddress.Type == corev1.NodeInternalIP {
				return nodeAddress.Address
			}
		}
		return ""
	})
}

func FilterPods(kubecli *kubernetes.Clientset, namespace, labelSelector, fieldSelector string, key func(p *corev1.Pod) string, values func(p *corev1.Pod) interface{}) (map[string]interface{}, error) {

	pods, err := kubecli.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})
	for _, p := range pods.Items {
		m[key(&p)] = values(&p)
	}
	return m, nil
}

func FilterNodes(kubecli *kubernetes.Clientset, filter func(node *corev1.Node) string) (map[string]string, error) {

	nodes, err := kubecli.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	nodeDict := make(map[string]string)
	for _, node := range nodes.Items {
		nodeDict[node.Name] = filter(&node)
	}
	return nodeDict, nil
}

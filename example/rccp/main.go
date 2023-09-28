package main

import (
	"context"
	"github.com/QQGoblin/go-sdk/example/rccp/rccpnuwa"
	"k8s.io/klog/v2"
)

func main() {

	nuwaCli, err := rccpnuwa.NewClient("172.28.117.100")
	if err != nil {
		klog.Fatal(err)
	}

	nodes, err := nuwaCli.List(context.Background())
	if err != nil {
		klog.Fatal(err)
	}
	for _, node := range nodes {
		klog.Infof("Node<%s> IP<%s> Role<%s>", node.Name, node.IP, node.Role)
	}

}

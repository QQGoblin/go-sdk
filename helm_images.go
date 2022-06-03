package main

import (
	"fmt"
	"github.com/QQGoblin/go-sdk/helm"
	"k8s.io/klog/v2"
	"os"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		klog.Fatal("Pelease select chart directory or tgz.")
	}
	images, err := helm.Images(args[1])
	if err != nil {
		klog.Fatal("Error.")
	}
	for _, image := range images {
		fmt.Println(image)
	}
}

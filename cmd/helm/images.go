package main

import (
	"fmt"
	helm2 "github.com/QQGoblin/go-sdk/pkg/helm"
	"k8s.io/klog/v2"
	"os"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		klog.Fatal("Pelease select chart directory or tgz.")
	}
	images, err := helm2.Images(args[1])
	if err != nil {
		klog.Fatal("Error.")
	}
	for _, image := range images {
		fmt.Println(image)
	}
}

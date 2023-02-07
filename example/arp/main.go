package main

import (
	"flag"
	"github.com/QQGoblin/go-sdk/pkg/network"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	addr string
	dev  string
)

func init() {
	flag.StringVar(&addr, "addr", "", "")
	flag.StringVar(&dev, "dev", "", "")
}

func main() {
	flag.Parse()

	if addr == "" || dev == "" {
		klog.Fatal("error address or device")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	network.ARPSendGratuitous(addr, dev, 1)

	// 运行主线程
	ticker := time.NewTicker(1100 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			klog.Infof("send gratuitous packets for address<%s>", addr)
			network.ARPSendGratuitous(addr, dev, 1)
		case <-c:
			ticker.Stop()
			goto END

		}
	}
END:
	klog.Info("exit arping process")
}

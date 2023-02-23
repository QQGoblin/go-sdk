package main

import (
	"flag"
	"github.com/QQGoblin/go-sdk/example/vipctrl/manager"
	"os"
	"os/signal"
	"syscall"
)

var (
	endpoints string
	timeout   string
	ttl       int
	interval  string

	address     string
	mask        string
	link        string
	MacVlanLink string
)

func init() {
	flag.StringVar(&endpoints, "endpoints", "https://127.0.0.1:2379", "ETCD 连接地址")
	flag.StringVar(&timeout, "timeout", "3s", "ETCD 连接超时时间")
	flag.IntVar(&ttl, "ttl", 5, "leader 租约超时时间")
	flag.StringVar(&interval, "interval", "3s", "leader 租约更新时间间隔")

	flag.StringVar(&address, "address", "172.28.112.11", "macvlan 设备的 ip 地址")
	flag.StringVar(&mask, "mask", "255.255.255.0", "子网掩码")
	flag.StringVar(&link, "link", "eth0", "macvlan 的父设备")
	flag.StringVar(&MacVlanLink, "macvlan", "eth0_wan0", "macvlan 设备名")
}

func main() {
	flag.Parse()

	controller := manager.NewController(endpoints, timeout, interval, ttl, address, mask, link, MacVlanLink)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	controller.Start()
	<-c
	controller.Stop()
}

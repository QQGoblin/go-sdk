package main

import (
	"flag"
	"github.com/QQGoblin/go-sdk/pkg/concurrency"
	"github.com/QQGoblin/go-sdk/pkg/network"
	"k8s.io/klog/v2"
	"net"
	"strconv"
	"time"
)

var (
	netScan  string
	portScan string
	ipScan   string
	timeout  string
)

func init() {
	flag.StringVar(&netScan, "net", "", "需要扫描的网段")
	flag.StringVar(&portScan, "port", "", "需要扫描的端口")
	flag.StringVar(&ipScan, "ip", "", "需要扫描的ip地址")
	flag.StringVar(&timeout, "timeout", "1s", "连接超时时间")
}

func main() {

	flag.Parse()

	t, err := time.ParseDuration(timeout)
	if err != nil {
		klog.Fatal("error param")
	}

	if ipScan != "" {
		scanHost(ipScan, t)
	} else {
		scanSubnet(netScan, portScan, t)
	}

}

func scanSubnet(subnet, port string, t time.Duration) {
	_, net, err := net.ParseCIDR(subnet)
	if err != nil {
		klog.Fatal("error cidr format")
	}
	wg := concurrency.NewWaitGroup(512)
	for _, ip := range network.SubnetAllIPs(net) {
		wg.BlockAdd()
		go func(ipStr, port string) {
			if connect(ipStr, port, t) == nil {
				klog.Infof("%s:%s is open", ipStr, port)
			}
			wg.Done()
		}(ip.String(), port)
	}
	wg.Wait()
}

func scanHost(ip string, t time.Duration) {
	wg := concurrency.NewWaitGroup(512)
	for port := 1024; port <= 65535; port++ {
		wg.BlockAdd()
		go func(ipStr, port string) {
			defer wg.Done()
			if connect(ipStr, port, t) == nil {
				klog.Infof("%s:%s is open", ipStr, port)
			}
		}(ip, strconv.Itoa(port))
	}
	wg.Wait()
}

func connect(ip, port string, t time.Duration) error {
	conn, err := net.DialTimeout("tcp", ip+":"+port, t)
	if err != nil {
		return err
	}

	defer conn.Close()
	return nil
}

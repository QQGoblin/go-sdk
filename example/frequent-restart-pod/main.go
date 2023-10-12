package main

import (
	"context"
	"flag"
	"github.com/QQGoblin/go-sdk/example/frequent-restart-pod/manager"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	namespace  string
	maxRestart int
	threshold  time.Duration
)

func init() {
	flag.StringVar(&namespace, "namespace", "default", "监听的命名空间")
	flag.IntVar(&maxRestart, "max-restart", 5, "重启次数阈值")
	flag.DurationVar(&threshold, "threshold", 15*time.Minute, "重启时间阈值")

}

func main() {

	flag.Parse()

	kubeconfig := flag.Lookup("kubeconfig").Value.String()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	ctx, cancel := context.WithCancel(context.Background())

	go manager.RunOrDie(ctx, kubeconfig, namespace, maxRestart, threshold)
	<-c
	cancel()
	return
}

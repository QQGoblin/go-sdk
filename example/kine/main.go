package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/QQGoblin/go-sdk/pkg/kine"
	"k8s.io/klog/v2"
	"strings"
)

var (
	action    string
	endpoints string
	cacert    string
	cert      string
	key       string
)

const (
	PutAction = "put"
	DelAction = "del"
	GetAction = "get"
)

func init() {
	flag.StringVar(&endpoints, "endpoints", "http://127.0.0.1:2379", "etcd service endpoints")
	flag.StringVar(&cacert, "cacert", "", "verify certificates of TLS-enabled secure servers using this CA bundle")
	flag.StringVar(&cert, "cert", "", "identify secure client using this TLS certificate file")
	flag.StringVar(&key, "key", "", "identify secure client using this TLS key file")
}

func main() {
	flag.Parse()
	args := flag.Args()
	if err := supports(args); err != nil {
		klog.Fatalf(err.Error())
	}

	kineCli, err := kine.New(strings.Split(endpoints, ","), cacert, cert, key)
	if err != nil {
		klog.Fatalf(err.Error())
	}
	defer kineCli.Close()

	switch args[0] {
	case PutAction:
		if err := kineCli.Put(context.Background(), args[1], []byte(args[2])); err != nil {
			klog.Fatalf(err.Error())
		}
		fmt.Println(args[1])
		fmt.Println(args[2])
	case GetAction:
		v, err := kineCli.Get(context.Background(), args[1])
		if err != nil {
			klog.Fatalf(err.Error())
		}
		fmt.Println(args[1])
		fmt.Println(string(v.Data))
	case DelAction:
		v, err := kineCli.Get(context.Background(), args[1])
		if err != nil {
			if errors.Is(err, kine.ErrNotFound) {
				return
			} else {
				klog.Fatalf(err.Error())
			}
		}
		if err := kineCli.Delete(context.Background(), args[1], v.Modified); err != nil {
			klog.Fatalf(err.Error())
		}
		fmt.Println(args[1])
	}

}

func supports(args []string) error {

	if len(args) == 0 || (args[0] != PutAction && args[0] != DelAction && args[0] != GetAction) {
		return errors.New("action is not supported")
	}

	if args[0] == PutAction && len(args) != 3 {
		return errors.New("please set key and values")
	}

	if (args[0] == GetAction || args[0] == DelAction) && len(args) != 2 {
		return errors.New("please set key")
	}

	return nil

}

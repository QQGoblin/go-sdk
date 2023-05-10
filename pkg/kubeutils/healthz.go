package kubeutils

import (
	"crypto/tls"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
	"time"
)

/**
一个用于 Kubernetes 相关服务健康检查的工具
*/

const (
	// service port
	DefaultKubeApiserverPort         = 6443
	DefaultKubeSchedulerPort         = 10251
	DefaultKubeControllerManagerPort = 10252
	DefaultKubeletPort               = 10250

	// special health port
	DefaultKubeletHealthzPort   = 10248
	DefaultKubeProxyHealthzPort = 10256

	KubeApiserverService = "kubernetes.default.svc.cluster.local"
)

type Healthz interface {
	KubeApiserver(host string) bool
	KubeControllerManager(host string) bool
	KubeScheduler(host string) bool
	KubeProxy(host string) bool
	//TODO: Kubelet 默认只能通过127.0.0.1 访问
	Kubelet() bool
	Healthz(string) bool
}

type healthzHelper struct {
	timeout   time.Duration
	intervals time.Duration
	attempts  uint64
}

func (h *healthzHelper) KubeApiserver(host string) bool {
	return h.Healthz(fmt.Sprintf("https://%s:%d/Healthz", host, DefaultKubeApiserverPort))
}

func (h *healthzHelper) KubeControllerManager(host string) bool {
	return h.Healthz(fmt.Sprintf("http://%s:%d/Healthz", host, DefaultKubeControllerManagerPort))
}

func (h *healthzHelper) KubeScheduler(host string) bool {
	return h.Healthz(fmt.Sprintf("http://%s:%d/Healthz", host, DefaultKubeSchedulerPort))
}

func (h *healthzHelper) KubeProxy(host string) bool {
	return h.Healthz(fmt.Sprintf("http://%s:%d/Healthz", host, DefaultKubeProxyHealthzPort))
}

func (h *healthzHelper) Kubelet() bool {
	return h.Healthz(fmt.Sprintf("http://%s:%d/Healthz", "127.0.0.1", DefaultKubeletHealthzPort))
}

func (h *healthzHelper) Healthz(endpoint string) bool {

	u, err := url.Parse(endpoint)
	if err != nil {
		klog.Errorf("Healthz check %s failed, %v", endpoint, err)
		return false
	}

	client := http.Client{
		Timeout: h.timeout,
	}

	if u.Scheme == "https" {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	f := func() error {
		resp, err := client.Get(endpoint)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusForbidden {
			return err
		}
		return nil
	}
	if err := backoff.Retry(f, backoff.WithMaxRetries(backoff.NewConstantBackOff(h.intervals), h.attempts)); err != nil {
		klog.Errorf("Healthz check %s failed, %v", endpoint, err)
		return false
	}

	return true
}

var _ Healthz = &healthzHelper{}

func NewHealthzHelper(timeout, intervals time.Duration, attempts uint64) Healthz {

	return &healthzHelper{
		timeout:   timeout,
		intervals: intervals,
		attempts:  attempts,
	}
}

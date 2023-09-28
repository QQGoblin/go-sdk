package kubeutils

import (
	"crypto/tls"
	"fmt"
	"github.com/QQGoblin/go-sdk/pkg/httputils"
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
	Kubelet() bool
}

type healthzHelper struct {
	timeout   time.Duration
	intervals time.Duration
	attempts  uint64
}

func (h *healthzHelper) KubeApiserver(host string) bool {
	return h.healthz(fmt.Sprintf("https://%s:%d/healthz", host, DefaultKubeApiserverPort))
}

func (h *healthzHelper) KubeControllerManager(host string) bool {
	return h.healthz(fmt.Sprintf("http://%s:%d/healthz", host, DefaultKubeControllerManagerPort))
}

func (h *healthzHelper) KubeScheduler(host string) bool {
	return h.healthz(fmt.Sprintf("http://%s:%d/healthz", host, DefaultKubeSchedulerPort))
}

func (h *healthzHelper) KubeProxy(host string) bool {
	return h.healthz(fmt.Sprintf("http://%s:%d/healthz", host, DefaultKubeProxyHealthzPort))
}

func (h *healthzHelper) Kubelet() bool {
	return h.healthz(fmt.Sprintf("http://%s:%d/healthz", "127.0.0.1", DefaultKubeletHealthzPort))
}

func (h *healthzHelper) healthz(endpoint string) bool {

	u, err := url.Parse(endpoint)

	if err != nil {
		return false
	}

	client := &http.Client{
		Timeout: h.timeout,
	}

	if u.Scheme == "https" {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	_, err = httputils.Healthz(client, endpoint, h.intervals, h.attempts)
	if err != nil {
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

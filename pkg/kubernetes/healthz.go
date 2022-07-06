package kubernetes

import (
	"crypto/tls"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
	"time"
)

const (
	// service port
	DefaultKubeApiserverPort         = 6443
	DefaultKubeSchedulerPort         = 10251
	DefaultKubeControllerManagerPort = 10252
	DefaultKubeletPort               = 10250

	// special health port
	DefaultKubeletHealthzPort   = 10248
	DefaultKubeProxyHealthzPort = 10256

	// heath timeout check
	DefaultHealthzCheckTimeout   = 5 * time.Second
	DefaultHealthzCheckIntervals = 5 * time.Second
	DefaultHealthzCheckAttempts  = 12
)

var (
	KubeApiserverHealthz         = fmt.Sprintf("https://127.0.0.1:%d/healthz", DefaultKubeApiserverPort)
	KubeControllerManagerHealthz = fmt.Sprintf("http://127.0.0.1:%d/healthz", DefaultKubeControllerManagerPort)
	KubeSchedulerHealthz         = fmt.Sprintf("http://127.0.0.1:%d/healthz", DefaultKubeSchedulerPort)
	KubeProxyHealthz             = fmt.Sprintf("http://127.0.0.1:%d/healthz", DefaultKubeProxyHealthzPort)
	KubeletHealthz               = fmt.Sprintf("http://127.0.0.1:%d/healthz", DefaultKubeletHealthzPort)
)

func Healthz() error {

	healthzEndpoints := []string{
		KubeApiserverHealthz,
		KubeControllerManagerHealthz,
		KubeSchedulerHealthz,
		KubeProxyHealthz,
		KubeletHealthz,
	}

	for _, ep := range healthzEndpoints {
		if err := healthz(ep); err != nil {
			klog.Error("check endpoint %s failed: %s", err.Error())
			return err
		}
		klog.Infof("check endpoint %s ready success", ep)
	}
	return nil
}

func healthz(endpoint string) error {

	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	client := http.Client{
		Timeout: DefaultHealthzCheckTimeout,
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
			klog.Warningf(err.Error())
			return err
		}
		if resp.StatusCode != http.StatusOK {
			err = errors.New(fmt.Sprintf("error check health for %s, http code %d", endpoint, resp.StatusCode))
			klog.Warningf(err.Error())
			return err
		}
		return nil
	}
	if err := backoff.Retry(f, backoff.WithMaxRetries(backoff.NewConstantBackOff(DefaultHealthzCheckIntervals), DefaultHealthzCheckAttempts)); err != nil {
		return err
	}

	return nil
}

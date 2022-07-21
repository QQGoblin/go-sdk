package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
	"time"
)

const (
	DefaultProtocol      = "https"
	DefaultClientPort    = 2379
	DefaultSyncInterval  = 60 * time.Second
	DefaultDialTimeout   = 5 * time.Second
	DefaultTimeout       = 15 * time.Second
	DefaultCertFile      = "client.crt"
	DefaultKeyFile       = "client.key"
	DefaultTrustedCAFile = "ca.crt"
)

type etcdV3Client struct {
	tlsConfig *tls.Config
	endpoints []string
	etcdCli   *clientv3.Client
	timeout   time.Duration
}

func New(ips ...string) (*etcdV3Client, error) {
	endpoints := make([]string, 0)

	if len(ips) > 0 {
		for _, ip := range ips {
			endpoints = append(endpoints, fmt.Sprintf("%s://%s:%d", DefaultProtocol, ip, DefaultClientPort))
		}
	} else {
		endpoints = append(endpoints, fmt.Sprintf("%s://%s:%d", DefaultProtocol, "127.0.0.1", DefaultClientPort))
	}

	tlsInfo := &transport.TLSInfo{
		CertFile:      DefaultCertFile,
		KeyFile:       DefaultKeyFile,
		TrustedCAFile: DefaultTrustedCAFile,
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, err
	}

	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:        endpoints,
		AutoSyncInterval: DefaultSyncInterval,
		DialTimeout:      DefaultDialTimeout,
		TLS:              tlsConfig,
	})
	if err != nil {
		return nil, err
	}

	return &etcdV3Client{
		tlsConfig: tlsConfig,
		endpoints: endpoints,
		etcdCli:   etcdCli,
		timeout:   DefaultTimeout,
	}, nil
}

func (c *etcdV3Client) WithTimeout(timeout time.Duration) *etcdV3Client {
	c.timeout = timeout
	return c
}

// BundleTlsConfig 从 string 中加载 tls.Config 信息
func BundleTlsConfig(caCertStr, certStr, keyStr string) (*tls.Config, error) {

	caCerts, err := cert.ParseCertsPEM([]byte(caCertStr))
	if err != nil {
		klog.Errorf("error parse CACert: %s", err)
		return nil, err
	}
	caCert := caCerts[0]

	tlsCert, err := tls.X509KeyPair([]byte(certStr), []byte(keyStr))
	if err != nil {
		klog.Errorf("error parse bundle cert and key: %s", err)
		return nil, err
	}

	cfg := tls.Config{
		Certificates: []tls.Certificate{
			tlsCert,
		},
		RootCAs:            x509.NewCertPool(),
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS12,
	}

	cfg.RootCAs.AddCert(caCert)
	return &cfg, nil
}

// Close 关闭 etcd 客户端连接
func (c *etcdV3Client) Close() error {
	if c.etcdCli == nil {
		return nil
	}
	return c.etcdCli.Close()
}

// Endpoint 检查所有 cli 中的 endpoint 是否是健康状态
func (c *etcdV3Client) EndpointHealth() (bool, error) {

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	resp, err := c.etcdCli.AlarmList(ctx)
	if err != nil {

		return false, err
	}

	if err == nil && len(resp.Alarms) > 0 {
		return false, nil
	}
	return true, nil
}

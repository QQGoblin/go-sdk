package etcd

import (
	"crypto/tls"
	"crypto/x509"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
	"time"
)

const (
	DefaultSyncInterval = 60 * time.Second
	DefaultDialTimeout  = 5 * time.Second
)

func NewClient(endpoints []string, ca, cert, key string) (*clientv3.Client, error) {

	tlsConfig, err := BundleTlsConfig(ca, cert, key)
	if err != nil {
		return nil, err
	}

	return clientv3.New(clientv3.Config{
		Endpoints:        endpoints,
		AutoSyncInterval: DefaultSyncInterval,
		DialTimeout:      DefaultDialTimeout,
		TLS:              tlsConfig,
	})
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

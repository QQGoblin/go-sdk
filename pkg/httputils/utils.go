package httputils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/cenkalti/backoff/v4"
	"io/ioutil"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
	"net"
	"net/http"
	"time"
)

const (
	DNSResolverTimeoutMS = 5000
	DNSResolverProto     = "udp"
)

func CreateTlsConfigFromFiles(caCertFile, certFile, keyFile string, insecureSkipVerify bool) (*tls.Config, error) {

	cert, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, err
	}

	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}

	if caCertFile == "" {
		return CreateTlsConfig(nil, cert, key, insecureSkipVerify)
	}

	caCert, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return nil, err
	}

	return CreateTlsConfig(caCert, cert, key, insecureSkipVerify)

}

func CreateTlsConfig(caCert, cert, key []byte, insecureSkipVerify bool) (*tls.Config, error) {

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		klog.Errorf("error parse bundle cert and key: %s", err)
		return nil, err
	}

	cfg := tls.Config{
		Certificates: []tls.Certificate{
			tlsCert,
		},
		RootCAs:            x509.NewCertPool(),
		InsecureSkipVerify: insecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS12,
	}

	if caCert != nil {
		caCerts, err := certutil.ParseCertsPEM(caCert)
		if err != nil {
			klog.Errorf("error parse CACert: %s", err)
			return nil, err
		}
		cfg.RootCAs.AddCert(caCerts[0])
	}

	return &cfg, nil
}

func CreateDialerWithResolver(resolverIP string) func(ctx context.Context, network string, addr string) (net.Conn, error) {

	dialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Duration(DNSResolverTimeoutMS) * time.Millisecond,
				}
				return d.DialContext(ctx, DNSResolverProto, resolverIP)
			},
		},
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}

}

func Healthz(cli *http.Client, endpoint string, interval time.Duration, attempts uint64) ([]byte, error) {

	var contents []byte

	f := func() error {
		resp, err := cli.Get(endpoint)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusForbidden {
			return err
		}

		contents, err = ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()

		return err
	}
	if err := backoff.Retry(f, backoff.WithMaxRetries(backoff.NewConstantBackOff(interval), attempts)); err != nil {
		return nil, err
	}

	return contents, nil
}

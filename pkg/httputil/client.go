package httputil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

func WithDNSResolverIP(c *http.Client, dnsResolverIP string) {

	dialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Duration(DNSResolverTimeoutMS) * time.Millisecond,
				}
				return d.DialContext(ctx, DNSResolverProto, dnsResolverIP)
			},
		},
	}

	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}

	if c.Transport == nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.DialContext = dialContext
		c.Transport = tr
	} else {
		c.Transport.(*http.Transport).DialContext = dialContext
	}
}

func WithTLSFiles(c *http.Client, caCertFile, certFile, keyFile string, insecureSkipVerify bool) error {

	cert, err := ioutil.ReadFile(certFile)
	if err != nil {
		return err
	}

	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return err
	}

	if caCertFile == "" {
		return WithTLSConfig(c, nil, cert, key, insecureSkipVerify)
	}

	caCert, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return err
	}

	return WithTLSConfig(c, caCert, cert, key, insecureSkipVerify)

}

func WithTLSConfig(c *http.Client, caCert, cert, key []byte, insecureSkipVerify bool) error {

	tlsConfig, err := CreateTlsConfig(caCert, cert, key, insecureSkipVerify)
	if err != nil {
		return err
	}

	if c.Transport == nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = tlsConfig
		c.Transport = tr
	} else {
		c.Transport.(*http.Transport).TLSClientConfig = tlsConfig
	}

	return nil
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

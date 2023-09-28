package rccpnuwa

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/QQGoblin/go-sdk/pkg/httputils"
	"time"
)

type client struct {
	httpClient *httputils.HTTPClient
	tlsConfig  *tls.Config
	timeout    time.Duration
	vip        string
	port       int
}

type Option func(c *client)

func WithTlsConfig(tlsConfig *tls.Config) Option {

	return func(c *client) {
		c.tlsConfig = tlsConfig
	}

}

func WithTimeout(timeout time.Duration) Option {

	return func(c *client) {
		c.timeout = timeout
	}

}

func NewClient(vip string, opts ...Option) (*client, error) {

	c := &client{
		vip:     vip,
		port:    9002,
		timeout: 180 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	var err error
	if c.tlsConfig == nil {
		c.tlsConfig, err = httputils.CreateTlsConfig([]byte(DefaultCACert), []byte(DefaultRCCPClientCert), []byte(DefaultRCCPClientKey), true)
		if err != nil {
			return nil, err
		}
	}

	c.httpClient = httputils.NewHTTPClient(
		httputils.WithTlsClientConfig(c.tlsConfig),
		httputils.WithCodec(&RCCPCodec{}),
		httputils.WithTimeout(c.timeout),
	)

	return c, nil
}

func (c *client) List(ctx context.Context) ([]*Node, error) {

	url := fmt.Sprintf("https://%s:%d/v1/nodes", c.vip, c.port)

	nodeList := &NodeList{}

	if err := c.httpClient.Get(ctx, url, nodeList); err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

func (c *client) Get(ctx context.Context, name string) (*Node, error) {

	url := fmt.Sprintf("https://%s:%d/v1/nodes/%s", c.vip, c.port, name)

	node := &Node{}

	if err := c.httpClient.Get(ctx, url, node); err != nil {
		return nil, err
	}
	return node, nil
}

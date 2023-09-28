package httputils

import (
	"context"
	"github.com/pkg/errors"
	"net/http"
	"os"
)

type HTTPClient struct {
	config *httpConfig
	*http.Client
}

func NewHTTPClient(opts ...HTTPOption) *HTTPClient {

	config := &httpConfig{}
	config.setDefault()

	for _, opt := range opts {
		opt(config)
	}

	transport := &http.Transport{
		DialContext:           config.DialContext,
		MaxConnsPerHost:       config.MaxIdleConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		TLSClientConfig:       config.TlsConfig,
	}
	return &HTTPClient{
		Client: &http.Client{Transport: transport},
		config: config,
	}
}

func (c *HTTPClient) invoke(ctx context.Context, url string, method string, content interface{}, reply interface{}, opts ...CallOption) error {

	callInfo := defaultCallInfo()
	for _, opt := range opts {
		opt(callInfo)
	}

	req, encodeErr := c.config.Codec.Encode(ctx, url, method, content, callInfo)
	if encodeErr != nil {
		return errors.Wrap(encodeErr, "encode request")
	}
	res, reqErr := c.Do(req)
	if reqErr != nil && os.IsTimeout(reqErr) {
		return errors.Wrap(reqErr, "request timeout")
	}
	if reqErr != nil {
		return errors.Wrap(reqErr, "request error")
	}
	return c.config.Codec.Decode(res, reply)
}
func (c *HTTPClient) Invoke(ctx context.Context, url string, method string, content interface{}, reply interface{}, opts ...CallOption) error {
	return c.invoke(ctx, url, method, content, reply, opts...)
}

func (c *HTTPClient) Delete(ctx context.Context, url string, content interface{}, reply interface{}, opts ...CallOption) error {
	return c.invoke(ctx, url, http.MethodDelete, content, reply, opts...)
}

func (c *HTTPClient) Get(ctx context.Context, url string, reply interface{}, opts ...CallOption) error {
	return c.invoke(ctx, url, http.MethodGet, nil, reply, opts...)
}

func (c *HTTPClient) Post(ctx context.Context, url string, content interface{}, reply interface{}, opts ...CallOption) error {
	return c.invoke(ctx, url, http.MethodPost, content, reply, opts...)
}

func (c *HTTPClient) Put(ctx context.Context, url string, content interface{}, reply interface{}, opts ...CallOption) error {
	return c.invoke(ctx, url, http.MethodPut, content, reply, opts...)
}

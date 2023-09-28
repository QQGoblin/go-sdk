package httputils

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

type CallInfo struct {
	Cookie map[string]string
	Header map[string]string
}

const (
	ContentTypeKey   = "Content-Type"
	UserAgentKey     = "User-Agent"
	UserAgentVersion = "GoSDK/1.0.0"
	ContentTypeJSON  = "application/json;charset=utf-8"
)

type CallOption func(info *CallInfo)

func defaultCallInfo() *CallInfo {

	header := map[string]string{
		UserAgentKey: UserAgentVersion,
	}

	return &CallInfo{
		Cookie: nil,
		Header: header,
	}
}

func WithHeader(header map[string]string) CallOption {
	return func(info *CallInfo) {
		if info.Header == nil {
			info.Header = make(map[string]string)
		}
		for k, v := range header {
			info.Header[k] = v
		}
	}
}

func WithCookie(cookie map[string]string) CallOption {
	return func(info *CallInfo) {
		if info.Cookie == nil {
			info.Cookie = make(map[string]string)
		}
		for k, v := range cookie {
			info.Cookie[k] = v
		}
	}
}

type httpConfig struct {
	Codec                 Codec
	Timeout               time.Duration
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	DialTimeout           time.Duration
	DialKeepAlive         time.Duration
	TlsConfig             *tls.Config
	ResolverAddress       string
	DialContext           func(ctx context.Context, network, address string) (net.Conn, error)
}

func (p *httpConfig) setDefault() {

	defaultDialer := &net.Dialer{
		Timeout:   p.DialTimeout,
		KeepAlive: p.DialKeepAlive,
	}

	p.Codec = &JSONCodec{}
	p.Timeout = 60 * time.Second
	p.MaxIdleConnsPerHost = 200
	p.IdleConnTimeout = 90 * time.Second
	p.TLSHandshakeTimeout = 10 * time.Second
	p.ExpectContinueTimeout = 1 * time.Second
	p.DialTimeout = 3 * time.Second
	p.DialKeepAlive = 5 * time.Second
	p.DialContext = defaultDialer.DialContext
}

type HTTPOption func(config *httpConfig)

func WithTimeout(timeout time.Duration) HTTPOption {
	return func(config *httpConfig) {
		config.Timeout = timeout
	}
}

func WithMaxIdleConnsPerHost(count int) HTTPOption {
	return func(config *httpConfig) {
		config.MaxIdleConnsPerHost = count
	}
}
func WithIdleConnTimeout(timeout time.Duration) HTTPOption {
	return func(config *httpConfig) {
		config.IdleConnTimeout = timeout
	}
}

func WithTLSHandshakeTimeout(timeout time.Duration) HTTPOption {
	return func(config *httpConfig) {
		config.TLSHandshakeTimeout = timeout
	}
}

func WithExpectContinueTimeout(timeout time.Duration) HTTPOption {
	return func(config *httpConfig) {
		config.ExpectContinueTimeout = timeout
	}
}

func WithDialTimeout(timeout time.Duration) HTTPOption {
	return func(config *httpConfig) {
		config.DialTimeout = timeout
	}
}

func WithDialKeepAlive(timeout time.Duration) HTTPOption {
	return func(config *httpConfig) {
		config.DialKeepAlive = timeout
	}
}

func WithTlsClientConfig(tlsConfig *tls.Config) HTTPOption {
	return func(config *httpConfig) {
		config.TlsConfig = tlsConfig
	}
}

func WithCodec(codec Codec) HTTPOption {
	return func(config *httpConfig) {
		config.Codec = codec
	}
}

func WithDialContext(dialContext func(ctx context.Context, network, address string) (net.Conn, error)) HTTPOption {
	return func(config *httpConfig) {
		config.DialContext = dialContext
	}
}

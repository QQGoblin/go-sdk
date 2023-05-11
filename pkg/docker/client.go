package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type SocketClient struct {
	socket  string
	timeout time.Duration
	cli     *http.Client
}

func NewSocketClient(socket string, timeout time.Duration) (*SocketClient, error) {
	tr := new(http.Transport)
	tr.DisableCompression = true
	tr.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
		return net.DialTimeout("unix", socket, timeout)
	}

	client := &http.Client{Transport: tr}

	if _, ok := client.Transport.(http.RoundTripper); !ok {
		return nil, fmt.Errorf("unable to verify tls configuration, invalid transport: %v", client.Transport)
	}
	return &SocketClient{
		socket:  socket,
		timeout: timeout,
		cli:     client,
	}, nil
}

// Info 检查 docker daemon 是否在运行，参考：curl -XGET --unix-socket /var/run/docker.sock  -H 'Content-Type: application/json' http://localhost/info
func (c *SocketClient) Info() (*Info, error) {

	req, err := http.NewRequest("GET", "/info", nil)
	if err != nil {
		return nil, fmt.Errorf("error creat request")
	}
	req.URL.Scheme = "http"
	req.URL.Host = c.socket

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)

	info := &Info{}
	if err := json.Unmarshal(b, info); err != nil {
		return nil, fmt.Errorf("unable unmarshal resp")
	}

	return info, nil
}

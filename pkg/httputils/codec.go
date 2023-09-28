package httputils

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"io"
	"net/http"
)

type Codec interface {
	Encode(ctx context.Context, url string, method string, content interface{}, callInfo *CallInfo) (*http.Request, error)
	Decode(res *http.Response, reply interface{}) error
}

var _ Codec = (*JSONCodec)(nil)

type JSONCodec struct{}

func (d JSONCodec) Encode(ctx context.Context, url string, method string, content interface{}, callInfo *CallInfo) (*http.Request, error) {

	var (
		body []byte
		err  error
	)
	if content != nil {
		body, err = json.Marshal(content)
		if err != nil {
			return nil, errors.Wrap(err, "error request body")
		}
	}
	r, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(err, "build request")
	}

	r.Header.Set(ContentTypeKey, ContentTypeJSON)
	for k, v := range callInfo.Header {
		r.Header.Set(k, v)
	}

	for k, v := range callInfo.Cookie {
		r.AddCookie(&http.Cookie{
			Name:  k,
			Value: v,
		})
	}
	return r, err
}

func (d JSONCodec) Decode(res *http.Response, reply interface{}) error {

	data, err := io.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		return errors.Wrap(err, "error response")
	}
	if res.StatusCode < 200 && res.StatusCode >= 300 {
		return errors.Wrapf(err, "error code %d, data: %s", res.StatusCode, string(data))
	}
	err = json.Unmarshal(data, reply)
	if err != nil {
		return errors.Wrapf(err, "decode data %s error", string(data))
	}
	return nil
}

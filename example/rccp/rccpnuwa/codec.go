package rccpnuwa

import (
	"context"
	"fmt"
	"github.com/QQGoblin/go-sdk/pkg/httputils"
	"net/http"
)

type RCCPCodec struct {
	defaultJSONCode httputils.JSONCodec
}

var _ httputils.Codec = &RCCPCodec{}

func (r *RCCPCodec) Encode(ctx context.Context, url string, method string, content interface{}, callInfo *httputils.CallInfo) (*http.Request, error) {

	bodyWrap := map[string]interface{}{
		"content": content,
	}

	if callInfo.Cookie == nil {
		callInfo.Cookie = map[string]string{}
	}

	callInfo.Cookie["commsg"] = "True"
	if _, isExist := callInfo.Cookie["api_name"]; !isExist {
		callInfo.Cookie["api_name"] = ""
	}
	return r.defaultJSONCode.Encode(ctx, url, method, bodyWrap, callInfo)

}

type rccpRespone struct {
	Content *content `json:"content"`
}

type content struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func (r *RCCPCodec) Decode(res *http.Response, reply interface{}) error {

	replyWrap := &rccpRespone{Content: &content{Data: reply}}

	if err := r.defaultJSONCode.Decode(res, replyWrap); err != nil {
		return err
	}

	if replyWrap.Content.Code != 0 {
		return fmt.Errorf("error code %d, data: %s", replyWrap.Content.Code, replyWrap.Content.Message)
	}

	return nil
}

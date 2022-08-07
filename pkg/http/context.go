package http

import (
	"encoding/json"
	"io"
	"net/http"
)

type Context struct {
	W http.ResponseWriter // 接口
	R *http.Request       // 结构体
}

func NewContext(writer http.ResponseWriter, request *http.Request) *Context {
	return &Context{
		W: writer,
		R: request,
	}
}

func (c *Context) ReadJSON(obj interface{}) error {
	r := c.R
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, obj); err != nil {
		return err
	}
	return nil
}

func (c *Context) WriteJSON(code int, obj interface{}) error {

	c.W.WriteHeader(code)
	j, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = c.W.Write(j)
	return err
}

func (c *Context) StatusOK(obj interface{}) error {
	return c.WriteJSON(http.StatusOK, obj)
}

func (c *Context) StatusInternalServerError(obj interface{}) error {
	return c.WriteJSON(http.StatusInternalServerError, obj)
}

func (c *Context) StatusBadRequest(obj interface{}) error {
	return c.WriteJSON(http.StatusBadRequest, obj)
}

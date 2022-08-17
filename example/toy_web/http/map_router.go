package http

import (
	"fmt"
	"net/http"
)

var _ Handler = &handlerBasedOnMap{}

// HandlerBasedOnMap 基于 Map 的路由
type handlerBasedOnMap struct {
	handlers map[string]HandlerFunc
}

func (h *handlerBasedOnMap) ServeHTTP(c *Context) {
	key := h.key(c.R.Method, c.R.URL.Path)
	if handler, ok := h.handlers[key]; ok {
		handler(c)
	} else {
		c.W.WriteHeader(http.StatusNotFound)
		c.W.Write([]byte("Not Found"))
	}
}

func (h *handlerBasedOnMap) Route(method, pattern string, handlerFunc HandlerFunc) error {
	k := h.key(method, pattern)
	h.handlers[k] = handlerFunc
	return nil
}

func (h *handlerBasedOnMap) key(method, pattern string) string {
	return fmt.Sprintf("%s#%s", method, pattern)
}

func NewHandlerBaseOnMap() Handler {
	return &handlerBasedOnMap{handlers: make(map[string]HandlerFunc)}
}

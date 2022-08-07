package http

import "net/http"

type HandlerFunc func(context *Context)

type Routable interface {
	Route(method, pattern string, handlerFunc HandlerFunc)
}

type Server interface {
	Routable
	Start(address string) error
}

var _ Server = &sdkHttpServer{}

type sdkHttpServer struct {
	Name    string
	Handler Handler
	root    Filter
}

func (s *sdkHttpServer) Route(method, pattern string, handlerFunc HandlerFunc) {
	// TODO: 处理重复注册的问题
	s.Handler.Route(method, pattern, handlerFunc)
}

func (s *sdkHttpServer) Start(address string) error {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		c := NewContext(writer, request)
		s.root(c)
	})
	return http.ListenAndServe(address, nil)
}

func NewHttpServer(name string, builders ...FilterBuilder) Server {
	handler := NewHandlerBaseOnMap()

	// root 是责任链的根，用于处理真实的业务逻辑，位于整个 filter 调用链的最后一层
	var root = handler.ServeHTTP

	for i := len(builders) - 1; i >= 0; i-- {
		b := builders[i]
		root = b(root)
	}

	return &sdkHttpServer{
		Name:    name,
		Handler: handler,
		root:    root,
	}
}

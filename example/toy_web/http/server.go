package http

import (
	"net/http"
	"sync"
)

// Server HTTP 服务端的顶级抽象
type Server interface {
	Register(method, pattern string, handlerFunc HandlerFunc) error
	Start(address string) error
	Shutdown() error
}

var _ Server = &sdkHttpServer{}

type sdkHttpServer struct {
	Name    string
	Handler Handler
	root    Filter
	ctxPool sync.Pool
}

func (s *sdkHttpServer) Register(method, pattern string, handlerFunc HandlerFunc) error {
	// TODO: 处理重复注册的问题
	return s.Handler.Route(method, pattern, handlerFunc)
}

func (s *sdkHttpServer) Start(address string) error {
	return http.ListenAndServe(address, s)
}

func (s *sdkHttpServer) Shutdown() error {
	return nil
}

func (s *sdkHttpServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {

	ctx := s.ctxPool.Get().(*Context)
	defer func() {
		s.ctxPool.Put(ctx)
	}()
	ctx.W = writer
	ctx.R = request
	ctx.Params = make(map[string]string)
	s.root(ctx)
}

func NewHttpServer(name string) Server {
	return NewHttpServerWithFilter(name)
}

func NewSdkHttpServerWithFilterNames(name string, filterNames ...string) Server {

	builders := make([]FilterBuilder, 0, len(filterNames))
	for _, n := range filterNames {
		b := GetFilterBuilder(n)
		builders = append(builders, b)
	}

	return NewHttpServerWithFilter(name, builders...)
}

func NewHttpServerWithFilter(name string, builders ...FilterBuilder) Server {

	// 基于路由树路由
	handler := NewHandlerBasedOnTree()

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
		ctxPool: sync.Pool{New: func() interface{} {
			return &Context{}
		},
		},
	}
}

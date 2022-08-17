package http

// HandlerFunc 路由函数入口
type HandlerFunc func(context *Context)

// Handler 路由处理接口
type Handler interface {
	ServeHTTP(c *Context)
	Route(method, pattern string, handlerFunc HandlerFunc) error
}

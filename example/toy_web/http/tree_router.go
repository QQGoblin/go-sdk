package http

import (
	"net/http"
	"sort"
	"strings"
)

// 路由节点类型的枚举
const (

	// 根节点，只有根用这个
	nodeTypeRoot = iota

	// *
	nodeTypeAny

	// 路径参数
	nodeTypeParam

	// 正则
	nodeTypeReg

	// 静态，即完全匹配
	nodeTypeStatic
)

// 通配符匹配
const any = "*"

// 支持的 method 方法
var supportMethods = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodDelete,
	http.MethodPatch,
}

// matchFunc 承担两个职责：1.路径和当前Node是否匹配；2.必要的数据写入到 Context
type matchFunc func(path string, c *Context) bool

type node struct {
	path     string
	children []*node

	// 节点的匹配函数，根据节点类型不同
	matchFunc matchFunc

	// 当路径和路由匹配成功时，执行处理方法
	handler HandlerFunc

	//匹配到这个节点的pattern
	pattern  string
	nodeType int
}

// newRootNode 创建根节点
func newRootNode(method string) *node {
	return &node{
		children: make([]*node, 0),
		matchFunc: func(p string, c *Context) bool {
			panic("never call me")
		},
		nodeType: nodeTypeRoot,
		pattern:  method,
	}
}

// newAnyNode 创建通配符 * 节点
func newAnyNode() *node {
	return &node{
		matchFunc: func(p string, c *Context) bool {
			return true
		},
		nodeType: nodeTypeAny,
		pattern:  any,
	}
}

// newParamNode 创建路径参数节点
func newParamNode(path string) *node {
	paramName := path[1:] // 注册路由时，传入的 path 为 ":id"
	return &node{
		children: make([]*node, 0),
		matchFunc: func(p string, c *Context) bool { // 路由时传入的值为 "04276bd8-c0d0-4779-9ccd-21b19d84dd4c"
			if c != nil {
				c.Params[paramName] = p
			}
			return p != any
		},
		nodeType: nodeTypeParam,
		pattern:  path,
	}
}

// newStaticNode 创建一个静态节点
func newStaticNode(path string) *node {
	return &node{
		children: make([]*node, 0),
		matchFunc: func(p string, c *Context) bool {
			return path == p && p != "*"
		},
		nodeType: nodeTypeStatic,
		pattern:  path,
	}
}

func newNode(path string) *node {
	if path == "*" {
		return newAnyNode()
	}
	if strings.HasPrefix(path, ":") {
		return newParamNode(path)
	}
	return newStaticNode(path)
}

// HandlerBaseOnTree 基于树的路由
type handlerBaseOnTree struct {
	forest map[string]*node
}

// NewHandlerBasedOnTree 初始化路由
func NewHandlerBasedOnTree() Handler {
	forest := make(map[string]*node, len(supportMethods))
	for _, m := range supportMethods {
		// 对应到每一个 method 都有一棵路由书
		forest[m] = newRootNode(m)
	}
	return &handlerBaseOnTree{
		forest: forest,
	}
}

// ServeHTTP 从路由数中寻找匹配的节点，如果找到了执行节点注册的 func，如果没有找到返回 404
func (h *handlerBaseOnTree) ServeHTTP(c *Context) {

	handler, found := h.findRouter(c.R.Method, c.R.URL.Path, c)

	if !found {
		c.W.WriteHeader(http.StatusNotFound)
		_, _ = c.W.Write([]byte("Not Found"))
		return
	}
	handler(c)
}

// Route 注册 URL 即在根节点下添加 node
func (h *handlerBaseOnTree) Route(method, pattern string, handlerFunc HandlerFunc) error {

	if err := h.validatePattern(pattern); err != nil {
		return err
	}

	// 将pattern按照URL的分隔符切割，统一格式
	pattern = strings.Trim(pattern, "/")
	paths := strings.Split(pattern, "/")

	// cur 指向指定 method 树的根节点
	var cur *node
	cur, ok := h.forest[method]
	if !ok {
		return ErrorInvalidMethod
	}

	for index, path := range paths {
		// 获取匹配的子节点
		matchChild, found := h.findMatchChild(cur, path, nil)
		if found && matchChild.nodeType != nodeTypeAny {
			cur = matchChild
		} else {
			h.createSubTree(cur, paths[index:], handlerFunc)
			return nil
		}
	}
	// 1. 支持短路径，如：先注册了 /order/detail 地址，再注册 /order
	// 2. 同时支持 ”/“ 路径注册
	cur.handler = handlerFunc
	return nil
}

// findRouter 路由！
func (h *handlerBaseOnTree) findRouter(method string, path string, c *Context) (HandlerFunc, bool) {
	// 去除头尾可能有的/，然后按照/切割成段
	paths := strings.Split(strings.Trim(path, "/"), "/")

	// 找出对应 method 的根节点
	cur, ok := h.forest[method]
	if !ok {
		return nil, false
	}

	// 从根节点向下遍历
	for _, p := range paths {
		matchChild, found := h.findMatchChild(cur, p, c)
		if !found {
			return nil, false
		}
		cur = matchChild
	}

	if cur.handler == nil {
		// 排除短路径访问场景，如：注册了 /user/profile ，但访问 /user
		return nil, false
	}
	// 访问 ”/“ 地址
	return cur.handler, true
}

// validatePattern 校验 pattern 是否合法
func (h *handlerBaseOnTree) validatePattern(pattern string) error {

	// 校验 *，必须在最后一个，并且前一个字符必须是 /，如 /user/*/id、/user/id-* 这些 url 都是非法的

	pos := strings.Index(pattern, "*")
	if pos == -1 {
		return nil
	}
	// 找到了 *
	if pos != len(pattern)-1 || pattern[pos-1] != '/' {
		return ErrorInvalidRouterPattern
	}

	return nil
}

// findMatchChild 寻找匹配的子节点
func (h *handlerBaseOnTree) findMatchChild(cur *node, path string, c *Context) (*node, bool) {

	candidates := make([]*node, 0)
	for _, child := range cur.children {
		if child.matchFunc(path, c) {
			candidates = append(candidates, child)
		}
	}

	if len(candidates) == 0 {
		return nil, false
	}

	// 可能出现不同类型的子节点匹配当前路径，此时根据 nodeType 决定其优先级
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].nodeType < candidates[j].nodeType
	})
	return candidates[len(candidates)-1], true
}

// createSubTree 创建子节点树
func (h *handlerBaseOnTree) createSubTree(root *node, paths []string, handlerFunc HandlerFunc) {

	cur := root
	for _, path := range paths {
		node := newNode(path)
		cur.children = append(cur.children, node)
		cur = node
	}
	cur.handler = handlerFunc
}

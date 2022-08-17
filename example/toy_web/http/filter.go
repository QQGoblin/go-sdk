package http

import (
	"fmt"
	"time"
)

const (
	MetricsFilter = "MetricsFilter"
)

// FilterBuilder 责任链构造器
type FilterBuilder func(next Filter) Filter

// Filter 责任链入口函数
type Filter HandlerFunc

// MetricsFilterBuilder 统计请求时间
func MetricsFilterBuilder(next Filter) Filter {
	return func(context *Context) {
		startTime := time.Now().UnixNano()
		next(context)
		endTime := time.Now().UnixNano()
		fmt.Printf("run time: %d \n", endTime-startTime)
	}
}

// filter 字典
var builderMap = map[string]FilterBuilder{
	MetricsFilter: MetricsFilterBuilder,
}

func RegisterFilterBuilder(name string, builder FilterBuilder) {
	// TODO: 这里要考虑 filter 的重复注册问题，当前不允许覆盖 Filter
	if _, exist := builderMap[name]; exist {
		return
	}
	builderMap[name] = builder
}

func GetFilterBuilder(name string) FilterBuilder {
	return builderMap[name]
}

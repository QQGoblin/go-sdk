package http

import (
	"fmt"
	"time"
)

type FilterBuilder func(next Filter) Filter

type Filter HandlerFunc

func MetricsFilterBuilder(next Filter) Filter {
	return func(context *Context) {
		startTime := time.Now().UnixNano()
		next(context)
		endTime := time.Now().UnixNano()
		fmt.Printf("run time: &d \n", endTime-startTime)
	}
}

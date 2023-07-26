package concurrency

import "sync"

type WaitGroup struct {
	size      int
	pool      chan byte
	waitCount int64
	waitGroup sync.WaitGroup // 底层还是利用sync.WaitGroup去做并发控制
}

// NewWaitGroup 创建一个带有size的并发池 当size为<=0时，直接走sync.WaitGroup逻辑
func NewWaitGroup(size int) *WaitGroup {
	wg := &WaitGroup{
		size: size,
	}
	if size > 0 {
		wg.pool = make(chan byte, size)
	}
	return wg
}

// Done 代表一个并发结束
func (wg *WaitGroup) Done() {
	if wg.size > 0 {
		<-wg.pool
	}
	wg.waitGroup.Done()
}

// Wait 等待所有并发goroutine结束
func (wg *WaitGroup) Wait() {
	wg.waitGroup.Wait()
}

func (wg *WaitGroup) BlockAdd() {
	if wg.size > 0 {
		wg.pool <- 1
	}
	wg.waitGroup.Add(1)
}

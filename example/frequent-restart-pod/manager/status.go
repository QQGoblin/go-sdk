package manager

import (
	v1 "k8s.io/api/core/v1"
	"sync"
)

type podQueue struct {
	items   []*v1.Pod
	size    int
	count   int
	current int
	lock    sync.Mutex
}

func NewPodQueue(size int) *podQueue {
	return &podQueue{
		items:   make([]*v1.Pod, size),
		size:    size,
		current: -1,
	}
}

func (q *podQueue) Push(p *v1.Pod) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.count++
	q.current++
	if q.current >= q.size {
		for i := 1; i < q.current; i++ {
			q.items[i-1] = q.items[i]
		}
		q.current = q.size - 1
	}
	q.items[q.current] = p

}

func (q *podQueue) Size() int {
	return q.current + 1
}

func (q *podQueue) First() *v1.Pod {
	if q.current >= 0 {
		return q.items[0]
	}
	return nil
}

func (q *podQueue) Last() *v1.Pod {
	if q.current >= 0 {
		return q.items[q.current]
	}
	return nil
}

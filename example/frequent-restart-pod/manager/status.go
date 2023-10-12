package manager

import (
	v1 "k8s.io/api/core/v1"
	"sync"
)

type record struct {
	size           int
	restartCount   int
	restartHistroy []*v1.Pod
	index          int
	firstSeen      *v1.Pod
	lock           sync.Mutex
}

func NewRecord(size int) *record {
	return &record{
		restartHistroy: make([]*v1.Pod, size),
		size:           size,
		index:          -1,
	}
}

func (q *record) Push(p *v1.Pod) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.restartCount++
	q.index++
	if q.index >= q.size {
		for i := 1; i < q.index; i++ {
			q.restartHistroy[i-1] = q.restartHistroy[i]
		}
		q.index = q.size - 1
	}
	q.restartHistroy[q.index] = p

}

func (q *record) Size() int {
	return q.index + 1
}

func (q *record) First() *v1.Pod {
	if q.index >= 0 {
		return q.restartHistroy[0]
	}
	return nil
}

func (q *record) FirstSeen() *v1.Pod {
	return q.firstSeen
}
func (q *record) SetFirstSeen(p *v1.Pod) {
	q.firstSeen = p
}

func (q *record) Last() *v1.Pod {
	if q.index >= 0 {
		return q.restartHistroy[q.index]
	}
	return nil
}

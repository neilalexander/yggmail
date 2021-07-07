package smtpsender

import (
	"sync"
)

type fifoQueue struct {
	frames []interface{}
	count  int
	mutex  sync.Mutex
	notifs chan struct{}
}

func newFIFOQueue() *fifoQueue {
	q := &fifoQueue{
		notifs: make(chan struct{}),
	}
	return q
}

func (q *fifoQueue) push(frame interface{}) bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.frames = append(q.frames, frame)
	q.count++
	select {
	case q.notifs <- struct{}{}:
	default:
	}
	return true
}

func (q *fifoQueue) pop() (interface{}, bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.count == 0 {
		return nil, false
	}
	frame := q.frames[0]
	q.frames[0] = nil
	q.frames = q.frames[1:]
	q.count--
	if q.count == 0 {
		q.frames = nil
	}
	return frame, true
}

func (q *fifoQueue) wait() <-chan struct{} {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.count > 0 {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return q.notifs
}

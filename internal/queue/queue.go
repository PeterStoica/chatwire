package queue

import "sync"

type Queue[T any] struct {
	mu    sync.Mutex
	items []T
	ready chan struct{}
}

func New[T any]() *Queue[T] {
	return &Queue[T]{ready: make(chan struct{}, 1)}
}

func (q *Queue[T]) Push(item T) {
	q.mu.Lock()
	q.items = append(q.items, item)
	q.mu.Unlock()
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

func (q *Queue[T]) Take() []T {
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.items
	q.items = nil
	return items
}

func (q *Queue[T]) Ready() <-chan struct{} {
	return q.ready
}

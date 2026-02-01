package queuelib

import (
	"context"
	"errors"
)

type Queue[T any] struct {
	queue chan T
}

const (
	queueSize = 500
)

func New[T any]() Queue[T] {
	return Queue[T]{
		queue: make(chan T, queueSize),
	}
}

func (q *Queue[T]) Enqueue(ctx context.Context, job T) error {
	select {
	case q.queue <- job:
		return nil
	default:
		return errors.New("queue full")
	}

}

func (q *Queue[T]) Dequeue(ctx context.Context) (T, bool) {
	select {
	case job := <-q.queue:
		return job, true
	case <-ctx.Done():
		var zero T
		return zero, false
	}
}

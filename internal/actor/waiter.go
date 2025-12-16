package actor

import (
	"gas/internal/errs"
	"time"
)

func newChanWaiter[T any](timeout time.Duration) *chanWaiter[T] {
	f := new(chanWaiter[T])
	f.ch = make(chan T, 1)
	f.after = time.After(timeout)
	return f
}

type chanWaiter[T any] struct {
	ch    chan T
	after <-chan time.Time
}

func (w *chanWaiter[T]) Wait() (T, error) {
	var t T
	select {
	case e := <-w.ch:
		return e, nil
	case <-w.after:
		return t, errs.ErrWaiterTimeout
	}
}

func (w *chanWaiter[T]) Done(reply T) {
	// 使用 select 实现非阻塞发送，避免多次调用 Done 时阻塞
	select {
	case w.ch <- reply:
	default:
	}
}

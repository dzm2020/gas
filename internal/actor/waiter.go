package actor

import "time"

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

func (f *chanWaiter[T]) Wait() (T, error) {
	var t T
	select {
	case e := <-f.ch:
		return e, nil
	case <-f.after:
		return t, ErrWaiterTimeout
	}
}

func (f *chanWaiter[T]) Done(reply T) {
	// 使用 select 实现非阻塞发送，避免多次调用 Done 时阻塞
	select {
	case f.ch <- reply:
	default:
	}
}

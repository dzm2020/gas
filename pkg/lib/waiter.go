package lib

import (
	"errors"
	"time"
)

type IWaiter interface {
	Wait() error
	Done()
}

func NewChanWaiter(deadline int64) *ChanWaiter {
	f := new(ChanWaiter)
	f.ch = make(chan struct{}, 1)
	targetTime := time.Unix(deadline, 0)
	delay := targetTime.Sub(time.Now())
	f.after = time.After(delay)
	return f
}

type ChanWaiter struct {
	ch    chan struct{}
	after <-chan time.Time
}

func (w *ChanWaiter) Wait() error {
	select {
	case <-w.ch:
		return nil
	case <-w.after:
		return errors.New("waiter timeout")
	}
}

func (w *ChanWaiter) Done() {
	// 使用 select 实现非阻塞发送，避免多次调用 Done 时阻塞
	select {
	case w.ch <- struct{}{}:
	default:
	}
}

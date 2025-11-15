package actor

import (
	"errors"
	"time"
)

func newChanWaiter(timeout time.Duration) *chanWaiter {
	f := new(chanWaiter)
	f.ch = make(chan error, 1)
	f.after = time.After(timeout)
	return f
}

type chanWaiter struct {
	ch    chan error
	after <-chan time.Time
}

func (f *chanWaiter) Wait() error {
	select {
	case e := <-f.ch:
		return e
	case <-f.after:
		return errors.New("timeout")
	}
}

func (f *chanWaiter) Done(err error) {
	f.ch <- err
}

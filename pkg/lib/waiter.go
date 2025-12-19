package lib

import (
	"errors"
	"time"
)

var errTimeout = errors.New("timeout")

func NewChanWaiter[T any](timeout time.Duration) *ChanWaiter[T] {
	f := new(ChanWaiter[T])
	f.dataChan = make(chan T, 1)
	f.errChan = make(chan error, 1)
	f.after = time.After(timeout)
	return f
}

type ChanWaiter[T any] struct {
	dataChan chan T
	errChan  chan error
	after    <-chan time.Time
}

func (w *ChanWaiter[T]) Wait() (t T, e error) {
	select {
	case t = <-w.dataChan:
		return t, nil
	case <-w.after:
		e = errTimeout
		return
	case e = <-w.errChan:
		return t, e
	}
}

func (w *ChanWaiter[T]) Done(rsp interface{}, err error) {
	if err != nil {
		w.doneErr(err)
		return
	}
	select {
	case w.dataChan <- rsp:
	default:
	}
}

func (w *ChanWaiter[T]) doneErr(err error) {
	select {
	case w.errChan <- err:
	default:
	}
}

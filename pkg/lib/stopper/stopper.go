package stopper

import "sync/atomic"

type Stopper struct {
	isStopped atomic.Bool
}

func (s *Stopper) IsStop() bool {
	return s.isStopped.Load()
}

func (s *Stopper) Stop() bool {
	return s.isStopped.CompareAndSwap(false, true)
}

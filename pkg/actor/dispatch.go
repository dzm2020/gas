// Package actor
// @Description:

package actor

type IDispatcher interface {
	Schedule(f func(), recoverFun func(err interface{})) error
	Throughput() int
}

// 协程调度器
type goroutineDispatcher int

func NewDefaultDispatcher(throughput int) IDispatcher {
	return goroutineDispatcher(throughput)
}
func (goroutineDispatcher) Schedule(fn func(), recoverFun func(err interface{})) error {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				recoverFun(err)
			}
		}()
		fn()
	}()
	return nil
}

func (d goroutineDispatcher) Throughput() int {
	return int(d)
}

// 同步调度器
type synchronizedDispatcher int

func (synchronizedDispatcher) Schedule(fn func(), recoverFun func(err interface{})) error {
	defer func() {
		if err := recover(); err != nil {
			recoverFun(err)
		}
	}()
	fn()
	return nil
}

func (d synchronizedDispatcher) Throughput() int {
	return int(d)
}

func NewSynchronizedDispatcher(throughput int) IDispatcher {
	return synchronizedDispatcher(throughput)
}

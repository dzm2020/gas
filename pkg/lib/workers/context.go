// Package workers
// @Description: 将 WaitGroup（等待协程完成）和 Context（取消协程）结合，并增加父子层级，解决 “层级化协程管理” 的问题；
package workers

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func WithWaitContext(parent *WaitContext) *WaitContext {
	wc := &WaitContext{
		WaitGroup: new(sync.WaitGroup),
		parent:    parent,
	}
	if parent != nil {
		wc.ctx, wc.cancel = context.WithCancel(parent.Context())
		parent.Add(1)
	} else {
		wc.ctx, wc.cancel = context.WithCancel(context.Background())
	}
	return wc
}

func WithContext(ctx context.Context) *WaitContext {
	wc := &WaitContext{
		WaitGroup: new(sync.WaitGroup),
	}
	wc.ctx, wc.cancel = context.WithCancel(ctx)
	return wc
}

type WaitContext struct {
	parent *WaitContext
	*sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func (wc *WaitContext) IsFinish() <-chan struct{} {
	return wc.ctx.Done()
}

func (wc *WaitContext) Cancel() {
	wc.cancel()
}

func (wc *WaitContext) Wait() {
	//  一直等
	wc.WaitGroup.Wait()

	if wc.parent == nil {
		return
	}
	wc.parent.Done()
	return
}

func (wc *WaitContext) WaitWithTimeout(timeout time.Duration) (err error) {
	//  带超时的等待
	done := make(chan struct{})
	go func() {
		wc.WaitGroup.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		err = fmt.Errorf("等待协程退出超时（%v），部分协程可能未完成清理", timeout)
	}

	if wc.parent == nil {
		return
	}
	wc.parent.Done()
	return
}

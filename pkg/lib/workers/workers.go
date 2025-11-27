/**
 * @Author: dingQingHui
 * @Description:
 * @File: workers
 * @Version: 1.0.0
 * @Date: 2025/1/2 10:16
 */

package workers

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
)

var (
	goCount    atomic.Int64
	panicCount atomic.Uint64
	pool       *ants.Pool
)

func init() {
	pool, _ = ants.NewPool(5000)
}

func Submit(fn func(), recoverFun func(err interface{})) {
	err := pool.Submit(func() {
		goCount.Add(1)
		Try(fn, recoverFun)
		goCount.Add(-1)
	})
	if err != nil {
		return
	}
}

func Try(fn func(), reFun func(err interface{})) {
	defer func() {
		if err := recover(); err != nil {
			panicCount.Add(1)
			if reFun != nil {
				reFun(err)
			}
		}
	}()
	fn()
}

var (
	ctx          = WithContext(context.Background())
	panicHandler func(interface{})
	isShutdown   atomic.Bool
)

func Go(f func(ctx *WaitContext)) {
	ctx.Add(1) // 启动前Add，避免竞态
	go func() {
		defer func() {
			// 无论是否panic，都标记Done
			ctx.Done()
			// 捕获panic，避免单个协程崩溃影响整体
			if r := recover(); r != nil {
				panicHandler(r)
			}
		}()
		f(ctx) // 传入上下文，供业务监听退出
	}()
}

func SetPanicHandler(handler func(interface{})) {
	panicHandler = handler
}

func Shutdown(timeout time.Duration) error {
	if !isShutdown.CompareAndSwap(false, true) {
		return nil
	}
	done := make(chan struct{})
	go func() {
		ctx.Wait()
		close(done)
	}()
	select {
	case <-done:
		fmt.Println("[INFO] 所有业务协程已退出")
	case <-time.After(timeout):
		return fmt.Errorf("等待协程退出超时（%v），部分协程可能未完成清理", timeout)
	}
	return nil
}

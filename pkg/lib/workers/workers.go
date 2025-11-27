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
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
)

var ()

func init() {
	pool, _ = ants.NewPool(5000)
}

var (
	group        sync.WaitGroup
	ctx, cancel  = context.WithCancel(context.Background())
	panicHandler func(interface{})
	isShutdown   atomic.Bool
	goCount      atomic.Int64
	panicCount   atomic.Uint64
	pool         *ants.Pool
)

func Go(f func(ctx context.Context)) {
	group.Add(1) // 启动前Add，避免竞态
	goCount.Add(1)
	go func() {
		defer func() {
			goCount.Add(-1)
			// 无论是否panic，都标记Done
			group.Done()
			// 捕获panic，避免单个协程崩溃影响整体
			if r := recover(); r != nil {
				panicCount.Add(1)
				panicHandler(r)
			}
		}()
		f(ctx) // 传入上下文，供业务监听退出
	}()
}

func GoTry(f func(ctx context.Context), try func(_ any)) {
	group.Add(1) // 启动前Add，避免竞态
	goCount.Add(1)
	go func() {
		defer func() {
			goCount.Add(-1)
			// 无论是否panic，都标记Done
			group.Done()
			// 捕获panic，避免单个协程崩溃影响整体
			if r := recover(); r != nil {
				panicCount.Add(1)
				try(r)
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
	cancel()
	done := make(chan struct{})
	go func() {
		group.Wait()
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

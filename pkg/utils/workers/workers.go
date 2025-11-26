/**
 * @Author: dingQingHui
 * @Description:
 * @File: workers
 * @Version: 1.0.0
 * @Date: 2025/1/2 10:16
 */

package workers

import (
	"sync/atomic"

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

package actor

import "gas/internal/iface"

// TaskMiddleware 定义了Task的中间件
// 每个中间件接收下一个Task并返回一个新的Task
type TaskMiddleware func(iface.Task) iface.Task

// chain 将已注册的中间件应用到目标Task上
// 若没有注册任何中间件，则原样返回task
func chain(middlewares []TaskMiddleware, task iface.Task) iface.Task {
	if task == nil {
		return nil
	}
	if len(middlewares) == 0 {
		return task
	}
	wrapped := task
	for i := len(middlewares) - 1; i >= 0; i-- {
		mw := middlewares[i]
		if mw == nil {
			continue
		}
		wrapped = mw(wrapped)
	}
	return wrapped
}

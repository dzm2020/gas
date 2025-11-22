package actor

import "gas/internal/iface"

// chain 将已注册的中间件应用到目标Task上
// 若没有注册任何中间件，则原样返回task
func chain(middlewares []iface.TaskMiddleware, task iface.Task) iface.Task {
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

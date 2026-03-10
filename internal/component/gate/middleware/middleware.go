// Package middleware 提供网关消息处理链：解码后（AfterDecode）与编码前（BeforeEncode）对 protocol.Message 进行修改或拦截。
// IMiddleware 定义在 gate/iface，本包仅提供实现与 RunAfterDecode/RunBeforeEncode，依赖 gateiface.IAgent 以保持类型约束。
package middleware

import (
	"github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
)

// RunAfterDecode 按顺序执行 chain 中每个中间件的 AfterDecode；任一返回 error 或 nil msg 即终止并返回。
func RunAfterDecode(chain []gateiface.IMiddleware, agent gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	var err error
	for _, mw := range chain {
		if mw == nil {
			continue
		}
		msg, err = mw.AfterDecode(agent, msg)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			return nil, nil
		}
	}
	return msg, nil
}

// RunBeforeEncode 按顺序执行 chain 中每个中间件的 BeforeEncode；任一返回 error 或 nil msg 即终止并返回。
func RunBeforeEncode(chain []gateiface.IMiddleware, agent gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	var err error
	for _, mw := range chain {
		if mw == nil {
			continue
		}
		msg, err = mw.BeforeEncode(agent, msg)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			return nil, nil
		}
	}
	return msg, nil
}

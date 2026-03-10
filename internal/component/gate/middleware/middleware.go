// Package middleware 提供网关消息处理链：解码后（AfterDecode）与编码前（BeforeEncode）对 protocol.Message 进行修改或拦截。
// @Description: IMiddleware 定义在 gate/iface，本包仅提供实现与 RunAfterDecode/RunBeforeEncode，依赖 gateiface.IAgent 以保持类型约束。

package middleware

import (
	"github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
)

// RunAfterDecode
//
//	@Description: 顺序执行 AfterDecode，任一 error 或 nil msg 即终止。
//	@param chain
//	@param agent
//	@param msg
//	@return *protocol.Message
//	@return error
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

// RunBeforeEncode
//
//	@Description: 顺序执行 BeforeEncode，任一 error 或 nil msg 即终止。
//	@param chain
//	@param agent
//	@param msg
//	@return *protocol.Message
//	@return error
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

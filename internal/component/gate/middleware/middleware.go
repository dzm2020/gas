package middleware

import (
	"github.com/dzm2020/gas/internal/component/gate/protocol"
)

// IMiddleware 在 codec.Decode 之后、以及 codec.Encode 之前对消息进行处理
type IMiddleware interface {
	// AfterDecode Decode 之后调用，可修改或替换 msg，返回 error 会终止后续处理
	AfterDecode(msg *protocol.Message) (*protocol.Message, error)
	// BeforeEncode Encode 之前调用，可修改或替换 msg，返回 error 会终止发送
	BeforeEncode(msg *protocol.Message) (*protocol.Message, error)
}

// RunAfterDecode 按顺序执行 middleware 的 AfterDecode
func RunAfterDecode(chain []IMiddleware, msg *protocol.Message) (*protocol.Message, error) {
	var err error
	for _, mw := range chain {
		if mw == nil {
			continue
		}
		msg, err = mw.AfterDecode(msg)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			return nil, nil
		}
	}
	return msg, nil
}

// RunBeforeEncode 按顺序执行 middleware 的 BeforeEncode
func RunBeforeEncode(chain []IMiddleware, msg *protocol.Message) (*protocol.Message, error) {
	var err error
	for _, mw := range chain {
		if mw == nil {
			continue
		}
		msg, err = mw.BeforeEncode(msg)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			return nil, nil
		}
	}
	return msg, nil
}

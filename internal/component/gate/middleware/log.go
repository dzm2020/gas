// 日志插件：在解码后与编码前记录消息摘要（Cmd/Act/Len），不修改消息。
package middleware

import (
	"github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/pkg/glog"

	"go.uber.org/zap"
)

// Log 日志中间件：AfterDecode 打 Debug 日志（收到），BeforeEncode 打 Debug 日志（发出）。
type Log struct{}

var _ gateiface.IMiddleware = (*Log)(nil)

// NewLog 创建日志中间件。
func NewLog() *Log {
	return &Log{}
}

func (l *Log) AfterDecode(_ gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	glog.Debug("gate.recv", zap.Uint8("cmd", msg.Cmd), zap.Uint8("act", msg.Act), zap.Uint32("len", msg.Len), zap.Uint8("tag", msg.Tag))
	return msg, nil
}

func (l *Log) BeforeEncode(_ gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	glog.Debug("gate.send", zap.Uint8("cmd", msg.Cmd), zap.Uint8("act", msg.Act), zap.Uint32("len", uint32(len(msg.Data))), zap.Uint8("tag", msg.Tag))
	return msg, nil
}

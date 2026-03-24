// Package session 提供基于 iface.ISession 的网关侧工具：编码回包、推送、Values 同步与断连。
//
// 约定：
//   - ISession 由连接侧 Agent 创建，业务 handler 收到的实例必须非 nil。
//   - 当前请求的协议帧由 SetMessage 写入会话（写入 pb.Session.Msg），跨节点后 GetMessage 可取回；业务回客户端优先用 Response/ResponseErr/Push，避免直接拼 Send。
//   - SetMessage 编码后的字节随 ActorMessage 传输，长度受 MaxSessionMessageBytes 限制。
package session

import (
	"encoding/json"
	"errors"

	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"go.uber.org/zap"
)

// MaxSessionMessageBytes 为会话内编码后的单帧客户端消息上限，避免过大负载随 ActorMessage 在集群中传递。
const MaxSessionMessageBytes = 512 * 1024

// 与对端 Agent 路由方法名一致，用于 Actor 消息的 method 字段。
const (
	sessionMethodPush     = "HandlerPush"
	sessionMethodSetValue = "HandlerSetValue"
	sessionMethodShutDown = "HandlerShutdown"
)

var (
	// ErrSessionIsNil 表示 ctx.Message() 为空或未携带 Session。
	ErrSessionIsNil = errors.New("session is nil")
	// ErrSessionMessageTooLarge 表示编码后的客户端消息超过 MaxSessionMessageBytes。
	ErrSessionMessageTooLarge = errors.New("session: encoded message exceeds MaxSessionMessageBytes")
)

// protocolMessageFromSession 从会话 Msg 解码当前请求协议消息（与 SetMessage 写入格式一致）。
func protocolMessageFromSession(s iface.ISession) *protocol.Message {
	bin := s.GetMessage()
	if len(bin) <= 0 {
		return nil
	}
	msg, _, err := codec.Decode(bin)
	if err != nil {
		glog.Warn("session.protocolMessageFromSession decode failed", zap.Error(err))
		return nil
	}
	return msg
}

// SetMessage 将当前请求协议消息编码后写入 s（pb.Session.Msg），供集群序列化后 GetMessage 使用。
// msg 为 nil 时会清空 Msg，避免沿用上一轮请求上下文。
func SetMessage(s iface.ISession, msg *protocol.Message) error {
	if s == nil {
		return ErrSessionIsNil
	}
	if msg == nil {
		s.SetMessage(nil)
		return nil
	}
	bin, err := codec.Encode(msg)
	if err != nil {
		glog.Warn("session.SetMessage encode failed", zap.Error(err))
		return err
	}
	if len(bin) > MaxSessionMessageBytes {
		return ErrSessionMessageTooLarge
	}
	s.SetMessage(bin)
	return nil
}

func SetValue(s iface.ISession, values map[string]string) error {
	if s == nil {
		return ErrSessionIsNil
	}
	bin, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return s.Send(sessionMethodSetValue, bin)
}

// Response 向对端推送业务响应（与 type Response 区分）。
func Response(s iface.ISession, data []byte) error {
	if s == nil {
		return ErrSessionIsNil
	}
	clientMsg := protocol.NewData(data)
	clientMsg.Copy(protocolMessageFromSession(s))
	bin, err := codec.Encode(clientMsg)
	if err != nil {
		return err
	}
	return s.Send(sessionMethodPush, bin)
}

// ResponseErr 设置错误码并推送无 body 消息。
func ResponseErr(s iface.ISession, errCode uint16) error {
	if s == nil {
		return ErrSessionIsNil
	}
	clientMsg := protocol.NewErr(errCode)
	clientMsg.Copy(protocolMessageFromSession(s))
	bin, err := codec.Encode(clientMsg)
	if err != nil {
		return err
	}
	return s.Send(sessionMethodPush, bin)
}

// Push 按 cmd/act 向对端推送带 body 消息。
func Push(s iface.ISession, cmd, act uint8, data []byte) error {
	if s == nil {
		return ErrSessionIsNil
	}
	clientMsg := protocol.New(cmd, act, data)

	bin, err := codec.Encode(clientMsg)
	if err != nil {
		return err
	}
	return s.Send(sessionMethodPush, bin)
}

// Shutdown 通知对端关闭连接（经 HandlerShutdown）。
func Shutdown(s iface.ISession) error {
	if s == nil {
		return ErrSessionIsNil
	}
	return s.Send(sessionMethodShutDown, nil)
}

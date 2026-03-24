package session

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"go.uber.org/zap"
)

// ClientMessage 为会话 Values 中存放当前请求协议消息（base64）的键，与 gate/session KeyMessage 一致。
const ClientMessage = "clientMessage"

// 与对端 Agent 路由方法名一致，用于 Actor 消息的 method 字段。
const (
	sessionMethodPush     = "HandlerPush"
	sessionMethodSetValue = "HandlerSetValue"
	sessionMethodShutDown = "HandlerShutdown"
)

// ErrSessionIsNil 表示 ctx.Message() 为空或未携带 Session。
var ErrSessionIsNil = errors.New("session is nil")

// protocolMessageFromSession 从 pb.Session.Values 解码当前请求协议消息（与网关 session 编码格式一致）。
func protocolMessageFromSession(s iface.ISession) *protocol.Message {
	value := s.GetString(ClientMessage)
	if value == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		raw = []byte(value)
	}
	msg, _, _ := codec.Decode(raw)
	return msg
}

// SetMessage
//
//	@Description: 设置当前请求消息并写入 Values（base64），供集群序列化后 GetMessage 使用。
//	@receiver a
//	@param msg
func SetMessage(s iface.ISession, msg *protocol.Message) error {
	if s == nil {
		return ErrSessionIsNil
	}
	if msg == nil {
		s.SetString(ClientMessage, "")
		return nil
	}
	value, err := codec.Encode(msg)
	if err != nil {
		glog.Warn("session.setMessageEncoded encode failed", zap.Error(err))
		s.SetString(ClientMessage, "")
		return nil
	}
	s.SetString(ClientMessage, base64.StdEncoding.EncodeToString(value))
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

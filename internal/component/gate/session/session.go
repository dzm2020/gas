package session

import (
	"encoding/base64"
	"errors"
	"strconv"

	"github.com/dzm2020/gas/api/pb"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"go.uber.org/zap"
)

// ClientMessage 为会话 Values 中存放当前请求协议消息（base64）的键，与 gate/session KeyMessage 一致。
const ClientMessage = "clientMessage"

// ErrSessionIsNil 表示 ctx.Message() 为空或未携带 Session。
var ErrSessionIsNil = errors.New("session is nil")

func ensureSessionPB(s *pb.Session) {
	if s == nil {
		return
	}
	if s.Values == nil {
		s.Values = make(map[string]string)
	}
}

// protocolMessageFromSessionPB 从 pb.Session.Values 解码当前请求协议消息（与网关 session 编码格式一致）。
func protocolMessageFromSessionPB(s *pb.Session) *protocol.Message {
	if s == nil || s.Values == nil {
		return nil
	}
	value := s.Values[ClientMessage]
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
func SetMessage(s *pb.Session, msg *protocol.Message) {
	if msg == nil {
		delete(s.Values, ClientMessage)
		return
	}
	value, err := codec.Encode(msg)
	if err != nil {
		glog.Warn("session.setMessageEncoded encode failed", zap.Error(err))
		delete(s.Values, ClientMessage)
		return
	}
	SetString(s, ClientMessage, base64.StdEncoding.EncodeToString(value))
}

func SetValue(ctx iface.IContext, s *pb.Session, values map[string]string) error {
	if s == nil {
		return ErrSessionIsNil
	}
	ensureSessionPB(s)
	trans := newTransport(ctx, s.GetAgent())
	return trans.setValue(values)
}

// Response 向对端推送业务响应（与 type Response 区分）。
func Response(ctx iface.IContext, s *pb.Session, data []byte) error {
	if s == nil {
		return ErrSessionIsNil
	}
	clientMsg := protocol.NewData(data)
	clientMsg.Copy(protocolMessageFromSessionPB(s))
	trans := newTransport(ctx, s.GetAgent())
	bin, err := codec.Encode(clientMsg)
	if err != nil {
		return xerror.Wrapf(err, "session response encode failed")
	}
	return trans.push(bin)
}

// ResponseErr 设置错误码并推送无 body 消息。
func ResponseErr(ctx iface.IContext, s *pb.Session, errCode uint16) error {
	if s == nil {
		return ErrSessionIsNil
	}
	clientMsg := protocol.NewErr(errCode)
	clientMsg.Copy(protocolMessageFromSessionPB(s))
	trans := newTransport(ctx, s.GetAgent())
	bin, err := codec.Encode(clientMsg)
	if err != nil {
		return xerror.Wrapf(err, "session.ResponseErr encode failed")
	}
	return trans.push(bin)
}

// Push 按 cmd/act 向对端推送带 body 消息。
func Push(ctx iface.IContext, s *pb.Session, cmd, act uint8, data []byte) error {
	if s == nil {
		return ErrSessionIsNil
	}
	clientMsg := protocol.New(cmd, act, data)
	trans := newTransport(ctx, s.GetAgent())
	bin, err := codec.Encode(clientMsg)
	if err != nil {
		return err
	}
	return trans.push(bin)
}

// Shutdown 通知对端关闭连接（经 HandlerShutdown）。
func Shutdown(ctx iface.IContext, s *pb.Session) error {
	if s == nil {
		return ErrSessionIsNil
	}
	trans := newTransport(ctx, s.GetAgent())
	return trans.closeRemote()
}

// SetString 设置 Values 中的字符串（不同步到对端）。
func SetString(s *pb.Session, key, value string) {
	ensureSessionPB(s)
	s.Values[key] = value
}

// GetString 从 Values 取字符串，不存在返回空串。
func GetString(s *pb.Session, key string) string {
	if s == nil || s.Values == nil {
		return ""
	}
	return s.Values[key]
}

// SetUint64 在 Values 中存 uint64
func SetUint64(s *pb.Session, key string, value uint64) {
	s.Values[key] = strconv.FormatUint(value, 10)
}

// GetUint64 从 Values 取并解析为 uint64，
func GetUint64(s *pb.Session, key string) uint64 {
	valStr, ok := s.Values[key]
	if !ok {
		return 0
	}
	val, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

// SetInt64 在 Values 中存 int64（不同步，需同步请调 SyncValues）。
func SetInt64(s *pb.Session, key string, value int64) {
	ensureSessionPB(s)
	s.Values[key] = strconv.FormatInt(value, 10)
}

// GetInt64 从 Values 取并解析为 int64，
func GetInt64(s *pb.Session, key string) int64 {
	valStr, ok := s.Values[key]
	if !ok {
		return 0
	}
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

// GetID 返回当前消息中的会话 Id
func GetID(s *pb.Session) int64 {
	return s.Id
}

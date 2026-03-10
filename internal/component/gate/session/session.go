package session

// Package session 提供网关侧会话封装：携带连接/请求的会话数据（Values、Agent 等），
// 并通过 ITransport 将 Response、Push、Close、SetValue 等操作下发到对端（如客户端连接、网关 Agent）。
// Message 在 Values 中以 base64 存储，避免集群 JSON 序列化时非法 UTF-8 导致 Index 错误。

import (
	"encoding/base64"
	"errors"
	"strconv"

	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/pb"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/xerror"

	"go.uber.org/zap"
)

const (
	KeyMessage = "clientMessage"
)

var (
	errTransportIsNil = errors.New("transport is nil")
)

func New(raw *pb.Session, ctx iface.IContext) *Session {
	s := &Session{
		Session: raw,
		transport: &transport{
			ctx:   ctx,
			agent: raw.GetAgent(),
		},
	}
	if s.Values == nil {
		s.Values = make(map[string]string)
	}
	return s
}

// Session 封装 *iface.Session 并提供 Response/Push/Close 等写能力，通过 transport 下发到对端。
type Session struct {
	*pb.Session // 原始数据
	transport   ITransport
	msg         *protocol.Message
}

// SetString 在 Values 中设置字符串，不经过 transport 同步。
func (a *Session) SetString(key, value string) {
	a.Values[key] = value
}

// GetString 从 Values 读取字符串，不存在返回空串。
func (a *Session) GetString(key string) string {
	return a.Values[key]
}

// SetUint64 在 Values 中以字符串形式存储 uint64，不触发 transport 同步；需同步时请调 SyncValues。
func (a *Session) SetUint64(key string, value uint64) {
	a.Values[key] = strconv.FormatUint(value, 10)
}

// GetUint64 从 Values 按 key 取字符串并解析为 uint64，不存在或解析失败返回 0。
func (a *Session) GetUint64(key string) uint64 {
	if a.Values == nil {
		return 0
	}
	valStr, ok := a.Values[key]
	if !ok {
		return 0
	}
	val, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

// SetInt64 在 Values 中以字符串形式存储 int64，不触发 transport 同步；需同步时请调 SyncValues。
func (a *Session) SetInt64(key string, value int64) {
	a.Values[key] = strconv.FormatInt(value, 10)
}

// GetInt64 从 Values 按 key 取字符串并解析为 int64，不存在或解析失败返回 0。
func (a *Session) GetInt64(key string) int64 {
	if a.Values == nil {
		return 0
	}
	valStr, ok := a.Values[key]
	if !ok {
		return 0
	}
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

// check 校验 transport 非空，写操作前调用。
func (a *Session) check() error {
	if a.transport == nil {
		return errTransportIsNil
	}
	return nil
}

// SyncValues 通过 transport 将当前 Values 同步到对端（如连接侧 Session）。
func (a *Session) SyncValues() error {
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.SetValue(a.Values)
}

// Response 向对端推送一条业务响应（走 Push 路由）。
func (a *Session) Response(data []byte) error {
	clientMsg := protocol.NewData(data)
	clientMsg.Copy(a.GetMessage())
	if err := a.check(); err != nil {
		return err
	}
	bin, err := codec.Encode(clientMsg)
	if err != nil {
		return xerror.Wrapf(err, "session response encode failed")
	}
	return a.transport.Push(bin)
}

// ResponseErr 设置 client.errCode 并向对端推送（无 body 的 Push）。
func (a *Session) ResponseErr(errCode uint16) error {
	clientMsg := protocol.NewErr(errCode)
	clientMsg.Copy(a.GetMessage())
	if err := a.check(); err != nil {
		return err
	}
	bin, err := codec.Encode(clientMsg)
	if err != nil {
		return xerror.Wrapf(err, "session.ResponseErr encode failed")
	}
	return a.transport.Push(bin)
}

// Push 设置 cmd/act 并向对端推送一条消息（带 body）。
func (a *Session) Push(cmd, act uint8, data []byte) error {
	clientMsg := protocol.New(cmd, act, data)
	if err := a.check(); err != nil {
		return err
	}
	bin, err := codec.Encode(clientMsg)
	if err != nil {
		return err
	}
	return a.transport.Push(bin)
}

// Close 通知对端关闭连接（Shutdown 路由）。
func (a *Session) Close() error {
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Close()
}

func (a *Session) GetAgent() *iface.Pid {
	return a.Agent
}

func (a *Session) Raw() *pb.Session {
	if a.msg != nil {
		a.setMessageEncoded(a.msg)
	}
	return a.Session
}

// setMessageEncoded 将 msg 编码为 base64 写入 Values[KeyMessage]，避免集群 JSON 序列化时
// 将非法 UTF-8 字节（如 Index 的 0xDE）替换为 U+FFFD(0xEF) 导致回复消息 Index 错误。
// Encode 失败时仅打日志并清除 KeyMessage，不中断调用方。
func (a *Session) setMessageEncoded(msg *protocol.Message) {
	if msg == nil {
		delete(a.Values, KeyMessage)
		return
	}
	value, err := codec.Encode(msg)
	if err != nil {
		glog.Warn("session.setMessageEncoded encode failed", zap.Error(err))
		delete(a.Values, KeyMessage)
		return
	}
	a.SetString(KeyMessage, base64.StdEncoding.EncodeToString(value))
}

// getMessageDecoded 从 Values[KeyMessage] 解码出 *protocol.Message，支持 base64 与原始字节（兼容旧数据）。
func (a *Session) getMessageDecoded() *protocol.Message {
	value := a.GetString(KeyMessage)
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

// SetMessage 设置当前请求的客户端消息，并同步写入 Values[KeyMessage]（base64），
// 确保经集群 JSON 序列化后 GetMessage 仍能拿到正确的 msg.Index。
func (a *Session) SetMessage(msg *protocol.Message) {
	a.msg = msg
	a.setMessageEncoded(msg)
}

func (a *Session) GetMessage() *protocol.Message {
	if a.msg == nil {
		a.msg = a.getMessageDecoded()
	}
	return a.msg
}

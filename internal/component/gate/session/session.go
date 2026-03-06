// Package session 提供网关侧会话封装：携带连接/请求的会话数据（Values、Agent 等），
// 并通过 ITransport 将 Response、Push、Close、SetValue 等操作下发到对端（如客户端连接，网关agent）。
package session

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/dzm2020/gas/internal/iface"
)

// Transport 方法名，与对端 Agent 路由一致。
const (
	MethodPush     = "Push"     // 推送消息到客户端
	MethodShutDown = "Shutdown" // 关闭连接
	MethodSetValue = "SetValue" // 同步 Values 到对端
)

// Values 中与协议/客户端消息相关的约定 key。
const (
	KeyClientMsgCmd     = "client.cmd"     // 命令字
	KeyClientMsgAct     = "client.act"     // 动作字
	KeyClientMsgErrCode = "client.errCode" // 错误码
	KeyClientMsgIndex   = "client.index"   // 序号
	KeyAgent            = "client.agent"   // Agent Pid 的 JSON 序列化
)

var (
	errTransportIsNil = errors.New("transport is nil")
)

// ITransport 将 Session 的写操作（Push/Shutdown/SetValue）转成对 Agent 的调用或系统消息。
type ITransport interface {
	Send(session iface.ISession, method string, payload interface{}) error
}

// New 仅用 id 构造 Session，transport 为 nil，仅可读；需写时由上层用 NewWithData 注入 transport。
func New(id int64) *Session {
	data := &iface.Session{
		Id:     id,
		Values: make(map[string]string),
	}
	return NewWithData(data, nil)
}

// NewWithData 用原始会话数据与 transport 构造 Session，用于处理请求时得到可写的 ISession。
func NewWithData(session *iface.Session, transport ITransport) *Session {
	s := &Session{
		Session:   session,
		transport: transport,
	}
	if s.Values == nil {
		s.Values = make(map[string]string)
	}
	return s
}

// Session 封装 *iface.Session 并提供 Response/Push/Close 等写能力，通过 transport 下发到对端。
type Session struct {
	*iface.Session            // 原始数据
	transport      ITransport // 写操作下发；nil 时仅可读
	agent          *iface.Pid // 缓存；为 nil 时 GetAgent 从 Values 反序列化
}

// Raw 返回底层 *iface.Session，供需要原始结构的调用方使用。
func (a *Session) Raw() *iface.Session {
	return a.Session
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
	return a.transport.Send(a, MethodSetValue, nil)
}

// Response 向对端推送一条业务响应（走 Push 路由）。
func (a *Session) Response(request interface{}) error {
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Send(a, MethodPush, request)
}

// ResponseCode 设置 client.errCode 并向对端推送（无 body 的 Push）。
func (a *Session) ResponseCode(code int64) error {
	a.SetInt64(KeyClientMsgErrCode, code)
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Send(a, MethodPush, nil)
}

// Push 设置 cmd/act 并向对端推送一条消息（带 body）。
func (a *Session) Push(cmd, act uint8, request interface{}) error {
	a.SetUint64(KeyClientMsgCmd, uint64(cmd))
	a.SetUint64(KeyClientMsgAct, uint64(act))
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Send(a, MethodPush, request)
}

// Close 通知对端关闭连接（Shutdown 路由）。
func (a *Session) Close() error {
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Send(a, MethodShutDown, nil)
}

// SetAgent 将 Pid JSON 序列化后写入 Values，并更新缓存。
func (a *Session) SetAgent(pid *iface.Pid) {
	a.agent = pid
	if pid == nil {
		delete(a.Values, KeyAgent)
		return
	}
	bin, _ := json.Marshal(pid)
	a.Values[KeyAgent] = string(bin)
}

// GetAgent 返回缓存的 agent；若为 nil 则从 Values 中 JSON 反序列化并缓存。
func (a *Session) GetAgent() *iface.Pid {
	if a.agent != nil {
		return a.agent
	}
	val, ok := a.Values[KeyAgent]
	if !ok || val == "" {
		return nil
	}
	var pid iface.Pid
	if err := json.Unmarshal([]byte(val), &pid); err != nil {
		return nil
	}
	a.agent = &pid
	return a.agent
}

// SetCmd 设置 client.cmd 并同步到对端。
func (a *Session) SetCmd(cmd uint8) {
	a.SetUint64(KeyClientMsgCmd, uint64(cmd))
}

// GetCmd 从 Values 读取 client.cmd。
func (a *Session) GetCmd() uint8 {
	return uint8(a.GetUint64(KeyClientMsgCmd))
}

// SetAct 设置 client.act 并同步到对端。
func (a *Session) SetAct(act uint8) {
	a.SetUint64(KeyClientMsgAct, uint64(act))
}

// GetAct 从 Values 读取 client.act。
func (a *Session) GetAct() uint8 {
	return uint8(a.GetUint64(KeyClientMsgAct))
}

// SetIndex 设置 client.index 并同步到对端。
func (a *Session) SetIndex(index uint32) {
	a.SetUint64(KeyClientMsgIndex, uint64(index))
}

// GetIndex 从 Values 读取 client.index。
func (a *Session) GetIndex() uint32 {
	return uint32(a.GetUint64(KeyClientMsgIndex))
}

// SetErrCode 设置 client.errCode 并同步到对端。
func (a *Session) SetErrCode(code int64) {
	a.SetInt64(KeyClientMsgErrCode, code)
}

// GetErrCode 从 Values 读取 client.errCode。
func (a *Session) GetErrCode() int64 {
	return a.GetInt64(KeyClientMsgErrCode)
}

// transport 实现 ITransport：将 Session 写操作转为发往 Agent 的 Actor 消息（本进程投递或 System.Send）。
type transport struct {
	ctx iface.IContext
}

// Send 序列化 payload，按 method 构造发往 session.GetAgent() 的消息并投递。
func (m *transport) Send(session iface.ISession, method string, payload interface{}) error {
	ses := session.(*Session)
	bin, err := m.ctx.Serializer().Marshal(payload)
	if err != nil {
		return err
	}
	msg := iface.NewActorMessage(m.ctx.ID(), ses.GetAgent(), method, bin)
	if ses.GetAgent() == m.ctx.ID() {
		return m.ctx.InvokerMessage(msg)
	} else {
		return m.ctx.System().Send(msg)
	}
}

// Package session 提供网关侧会话封装：携带连接/请求的会话数据（Values、Agent 等），
// 并通过 ITransport 将 Response、Push、Close、SetValue 等操作下发到对端（如客户端连接，网关agent）。
package session

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
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

// New 仅用 id 构造 Session，transport 为 nil，仅可读；需写时由上层用 NewWithData 注入 transport。
func New(id int64) *Session {
	data := &iface.Session{
		Id:     id,
		Values: make(map[string]string),
	}
	return NewWithData(data, nil)
}

// NewWithData 用原始会话数据与 transport 构造 Session，用于处理请求时得到可写的 ISession。
func NewWithData(data *iface.Session, transport ITransport) *Session {
	s := &Session{
		Session:   data,
		transport: transport,
	}
	if s.Values == nil {
		s.Values = make(map[string]string)
	}
	return s
}

// Session 封装 *iface.Session 并提供 Response/Push/Close 等写能力，通过 transport 下发到对端。
type Session struct {
	*iface.Session             // 原始数据
	transport      ITransport  // 写操作下发；nil 时仅可读
	agent          *iface.Pid  // 缓存；为 nil 时 GetAgent 从 Values 反序列化
	middlewareRef  interface{} // 指向当前连接对应 agent 的 middleware 链（*[]Middleware），由 gate 注入，避免循环依赖用 interface{}
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
	bin, _ := json.Marshal(a.Values)
	return a.transport.Send(a.GetAgent(), MethodSetValue, bin)
}

// Response 向对端推送一条业务响应（走 Push 路由）。
func (a *Session) Response(data []byte) error {
	clientMsg := protocol.New(a.GetCmd(), a.GetAct(), data)
	clientMsg.SetIndex(a.GetIndex())
	if err := a.check(); err != nil {
		return err
	}
	bin, _ := codec.Encode(clientMsg)
	return a.transport.Send(a.GetAgent(), MethodPush, bin)
}

// ResponseErr 设置 client.errCode 并向对端推送（无 body 的 Push）。
func (a *Session) ResponseErr(code uint16) error {
	clientMsg := protocol.New(a.GetCmd(), a.GetAct(), nil)
	clientMsg.SetIndex(a.GetIndex())
	clientMsg.SetError(code)
	if err := a.check(); err != nil {
		return err
	}
	bin, _ := codec.Encode(clientMsg)
	return a.transport.Send(a.GetAgent(), MethodPush, bin)
}

// Push 设置 cmd/act 并向对端推送一条消息（带 body）。
func (a *Session) Push(cmd, act uint8, data []byte) error {
	clientMsg := protocol.New(cmd, act, data)
	if err := a.check(); err != nil {
		return err
	}
	bin, _ := codec.Encode(clientMsg)
	return a.transport.Send(a.GetAgent(), MethodPush, bin)
}

// Close 通知对端关闭连接（Shutdown 路由）。
func (a *Session) Close() error {
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Send(a.GetAgent(), MethodShutDown, nil)
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

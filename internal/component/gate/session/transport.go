// Package session
// @Description: ITransport 将 Session 的写操作转成对 Agent 的 Actor 调用或系统消息。
package session

import (
	"encoding/json"

	"github.com/dzm2020/gas/internal/iface"
)

// 与对端 Agent 路由方法名一致，用于 Actor 消息的 method 字段。
const (
	MethodPush     = "HandlerPush"     // 推送消息到客户端
	MethodSetValue = "HandlerSetValue" // 同步 Values 到对端
	MethodShutDown = "HandlerShutdown" // 关闭连接
)

// ITransport
// @Description: ITransport 将 Session 的写操作转成对 Agent 的 Actor 调用或系统消息。
type ITransport interface {
	Push(bin []byte) error
	SetValue(values map[string]string) error
	Close() error
}

// transport
// @Description: transport 实现 ITransport：目标为 session 绑定的 Agent Pid，本进程内直接投递，跨节点则 System.Send。
type transport struct {
	ctx   iface.IContext // 正在处理session的actor context
	agent *iface.Pid     // session 的agent pid
}

// send
//
//	@Description:  send 按 method 构造发往 to 的 Actor 消息并投递；若 to 与当前 ctx 同进程则 InvokerMessage，否则 System.Send。
//	@receiver m
//	@param to
//	@param method
//	@param bin
//	@return error
func (m *transport) send(to *iface.Pid, method string, bin []byte) error {
	msg := iface.NewActorMessage(m.ctx.ID(), to, method, bin)
	if iface.EqualPid(to, m.ctx.ID()) {
		return m.ctx.InvokerMessage(msg)
	} else {
		return m.ctx.System().Send(msg)
	}
}

// SetValue
//
//	@Description:同步agent session values
//	@receiver m
//	@param values
//	@return error
func (m *transport) SetValue(values map[string]string) error {
	bin, _ := json.Marshal(values)
	return m.send(m.agent, MethodSetValue, bin)
}

// Push
//
//	@Description: 推送消息到客户端
//	@receiver m
//	@param bin
//	@return error
func (m *transport) Push(bin []byte) error {
	return m.send(m.agent, MethodPush, bin)
}

// Close
//
//	@Description: 关闭客户端
//	@receiver m
//	@return error
func (m *transport) Close() error {
	return m.send(m.agent, MethodShutDown, nil)
}

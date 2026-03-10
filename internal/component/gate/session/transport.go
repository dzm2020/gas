// Package session
// @Description: 本文件实现 ITransport：把 Session 的 Push/SetValue/Close 转成发往 Agent 的 Actor 调用（本进程 InvokerMessage 或跨actor System.Send）。
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

// ITransport 将 Session 的写操作（Push/SetValue/Close）转成对 Agent 的 Actor 调用或系统消息。
type ITransport interface {
	Push(bin []byte) error
	SetValue(values map[string]string) error
	Close() error
}

// transport 实现 ITransport：目标为 session 绑定的 Agent Pid，本进程内直接投递，跨节点则 System.Send。
type transport struct {
	ctx   iface.IContext
	agent *iface.Pid
}

// send 按 method 构造发往 to 的 Actor 消息并投递；若 to 与当前 ctx 同进程则 InvokerMessage，否则 System.Send。
func (m *transport) send(to *iface.Pid, method string, bin []byte) error {
	msg := iface.NewActorMessage(m.ctx.ID(), to, method, bin)
	if iface.EqualPid(to, m.ctx.ID()) {
		return m.ctx.InvokerMessage(msg)
	} else {
		return m.ctx.System().Send(msg)
	}
}

func (m *transport) SetValue(values map[string]string) error {
	bin, _ := json.Marshal(values)
	return m.send(m.agent, MethodSetValue, bin)
}

func (m *transport) Push(bin []byte) error {
	return m.send(m.agent, MethodPush, bin)
}

func (m *transport) Close() error {
	return m.send(m.agent, MethodShutDown, nil)
}

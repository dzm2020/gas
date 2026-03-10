package session

import (
	"encoding/json"

	"github.com/dzm2020/gas/internal/iface"
)

// Transport 方法名，与对端 Agent 路由一致。
const (
	MethodPush     = "Push"     // 推送消息到客户端
	MethodShutDown = "Shutdown" // 关闭连接
	MethodSetValue = "SetValue" // 同步 Values 到对端
)

// ITransport 将 Session 的写操作（Push/Shutdown/SetValue）转成对 Agent 的调用或系统消息。
type ITransport interface {
	Push(bin []byte) error
	SetValue(values map[string]string) error
	Close() error
}

// transport 实现 ITransport：将 Session 写操作转为发往 Agent 的 Actor 消息（本进程投递或 System.Send）。
type transport struct {
	ctx   iface.IContext
	agent *iface.Pid
}

// Send 序列化 payload，按 method 构造发往 session.GetAgent() 的消息并投递。
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

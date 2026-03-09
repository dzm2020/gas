package session

import "github.com/dzm2020/gas/internal/iface"

// ITransport 将 Session 的写操作（Push/Shutdown/SetValue）转成对 Agent 的调用或系统消息。
type ITransport interface {
	Send(to *iface.Pid, method string, bin []byte) error
}

// transport 实现 ITransport：将 Session 写操作转为发往 Agent 的 Actor 消息（本进程投递或 System.Send）。
type transport struct {
	ctx iface.IContext
}

// Send 序列化 payload，按 method 构造发往 session.GetAgent() 的消息并投递。
func (m *transport) Send(to *iface.Pid, method string, bin []byte) error {
	msg := iface.NewActorMessage(m.ctx.ID(), to, method, bin)
	if to.Equal(m.ctx.ID()) {
		return m.ctx.InvokerMessage(msg)
	} else {
		return m.ctx.System().Send(msg)
	}
}

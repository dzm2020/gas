package session

import (
	"encoding/json"

	"github.com/dzm2020/gas/internal/iface"
)

// 与对端 Agent 路由方法名一致，用于 Actor 消息的 method 字段。
const (
	sessionMethodPush     = "HandlerPush"
	sessionMethodSetValue = "HandlerSetValue"
	sessionMethodShutDown = "HandlerShutdown"
)

func newTransport(ctx iface.IContext, agent *iface.Pid) *transport {
	return &transport{ctx: ctx, agent: agent}
}

type transport struct {
	ctx   iface.IContext
	agent *iface.Pid
}

func (m *transport) send(to *iface.Pid, method string, bin []byte) error {
	msg := iface.NewActorMessage(m.ctx.ID(), to, method, bin)
	if iface.EqualPid(to, m.ctx.ID()) {
		return m.ctx.InvokerMessage(msg)
	}
	return m.ctx.System().SendMessage(msg)
}

func (m *transport) setValue(values map[string]string) error {
	bin, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return m.send(m.agent, sessionMethodSetValue, bin)
}

func (m *transport) push(bin []byte) error {
	return m.send(m.agent, sessionMethodPush, bin)
}

func (m *transport) closeRemote() error {
	return m.send(m.agent, sessionMethodShutDown, nil)
}

package session

import (
	"gas/internal/iface"

	"github.com/duke-git/lancet/v2/convertor"
)

const (
	PushMessageToClientMethod   = "PushMessageToClient"
	CloseClientConnectionMethod = "CloseClientConnection"
)

func New(session *iface.Session) *Session {
	return &Session{
		Session: session,
	}
}

type Session struct {
	*iface.Session
	ctx iface.IContext
}

func (a *Session) SetContext(ctx iface.IContext) {
	a.ctx = ctx
}

func (a *Session) Response(request interface{}) error {
	bin := iface.GetNode().Marshal(request)
	message := iface.NewActorMessage(a.ctx.ID(), a.GetAgent(), PushMessageToClientMethod, bin)
	message.Session = convertor.DeepClone(a.Session)
	return a.send(message)
}

func (a *Session) ResponseCode(code int64) error {
	message := iface.NewActorMessage(a.ctx.ID(), a.GetAgent(), PushMessageToClientMethod, nil)
	message.Session = convertor.DeepClone(a.Session)
	message.Session.Code = code
	return a.send(message)
}

func (a *Session) Push(cmd, act uint16, request interface{}) error {
	bin := iface.GetNode().Marshal(request)
	message := iface.NewActorMessage(a.ctx.ID(), a.GetAgent(), PushMessageToClientMethod, bin)
	message.Session = convertor.DeepClone(a.Session)
	message.Session.Cmd = uint32(cmd)
	message.Session.Act = uint32(act)
	return a.send(message)
}

// sendToSession 发送消息到会话，如果是本地则直接调用，否则通过系统发送
func (a *Session) send(message *iface.ActorMessage) error {
	if a.GetAgent() == a.ctx.ID() {
		return a.ctx.InvokerMessage(message)
	} else {
		node := iface.GetNode()
		system := node.System()
		return system.Send(message)
	}
}

func (a *Session) Close() error {
	message := iface.NewActorMessage(a.ctx.ID(), a.GetAgent(), CloseClientConnectionMethod, nil)
	return a.send(message)
}

package iface

import "github.com/duke-git/lancet/v2/convertor"

type ISession interface {
	GetMid() int64
	GetIndex() uint32
	GetCode() int64
	Response(request interface{}) error
	ResponseCode(code int64) error
	Forward(to interface{}, method string) error
	Push(msgId uint16, request interface{}) error
	GetEntityId() int64
}

const (
	PushMessageToClientMethod   = "PushMessageToClient"
	CloseClientConnectionMethod = "CloseClientConnection"
)

func NewSessionWithPid(ctx IContext, agent *Pid) ISession {
	return &WrapSession{
		Session: &Session{Agent: agent},
		ctx:     ctx,
	}
}
func NewSession(ctx IContext, session *Session) ISession {
	return &WrapSession{
		Session: session,
		ctx:     ctx,
	}
}

type WrapSession struct {
	*Session
	ctx IContext
}

func (a *WrapSession) Response(request interface{}) error {
	bin := GetNode().Marshal(request)
	message := NewActorMessage(a.ctx.ID(), a.GetAgent(), PushMessageToClientMethod, bin)
	message.Session = convertor.DeepClone(a.Session)
	return a.send(message)
}

func (a *WrapSession) ResponseCode(code int64) error {
	message := NewActorMessage(a.ctx.ID(), a.GetAgent(), PushMessageToClientMethod, nil)
	message.Session = convertor.DeepClone(a.Session)
	message.Session.Code = code
	return a.send(message)
}

func (a *WrapSession) Forward(to interface{}, method string) error {
	toPid := GetNode().CastPid(to)
	message := convertor.DeepClone(a.ctx.Message())
	message.To = toPid
	message.Method = method
	return GetNode().Send(message)
}

func (a *WrapSession) Push(msgId uint16, request interface{}) error {
	bin := GetNode().Marshal(request)
	message := NewActorMessage(a.ctx.ID(), a.GetAgent(), PushMessageToClientMethod, bin)
	message.Session = convertor.DeepClone(a.Session)
	message.Session.Mid = int64(msgId)
	return a.send(message)
}

// sendToSession 发送消息到会话，如果是本地则直接调用，否则通过系统发送
func (a *WrapSession) send(message *ActorMessage) error {
	if a.GetAgent() == a.ctx.ID() {
		return a.ctx.InvokerMessage(message)
	}
	return GetNode().Send(message)
}

func (a *WrapSession) Close() error {
	message := NewActorMessage(a.ctx.ID(), a.GetAgent(), CloseClientConnectionMethod, nil)
	return a.send(message)
}

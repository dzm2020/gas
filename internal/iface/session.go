package iface

import "github.com/duke-git/lancet/v2/convertor"

type ISession interface {
	GetMid() int64
	GetIndex() uint32
	GetCode() int64
	Response(request interface{}) error
	ResponseCode(code int64) error
	Forward(toPid *Pid, method string) error
	Push(msgId uint16, request interface{}) error
	GetEntityId() int64
}

const (
	PushMessageToClientMethod   = "PushMessageToClient"
	CloseClientConnectionMethod = "CloseClientConnection"
)

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
	message := NewActorMessage(a.ctx.ID(), a.GetAgent(), PushMessageToClientMethod)
	message.Session = convertor.DeepClone(a.Session)
	message.Data = a.ctx.Node().Marshal(request)
	return a.send(message)
}

func (a *WrapSession) ResponseCode(code int64) error {
	message := NewActorMessage(a.ctx.ID(), a.GetAgent(), PushMessageToClientMethod)
	message.Session = convertor.DeepClone(a.Session)
	message.Session.Code = code
	return a.send(message)
}

func (a *WrapSession) Forward(toPid *Pid, method string) error {
	message := convertor.DeepClone(a.ctx.Message())
	message.To = toPid
	message.Method = method
	return a.ctx.System().Send(message)
}

func (a *WrapSession) Push(msgId uint16, request interface{}) error {
	message := NewActorMessage(a.ctx.ID(), a.GetAgent(), PushMessageToClientMethod)
	message.Session = convertor.DeepClone(a.Session)
	message.Session.Mid = int64(msgId)
	message.Data = a.ctx.Node().Marshal(request)
	return a.send(message)
}

// sendToSession 发送消息到会话，如果是本地则直接调用，否则通过系统发送
func (a *WrapSession) send(message *ActorMessage) error {
	if a.GetAgent() == a.ctx.ID() {
		return a.ctx.InvokerMessage(message)
	}
	return a.ctx.System().Send(message)
}

func (a *WrapSession) Close() error {
	message := NewActorMessage(a.ctx.ID(), a.GetAgent(), CloseClientConnectionMethod)
	return a.send(message)
}

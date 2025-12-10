package iface

import "errors"

const (
	MsgIdPushMessageToClient   = -1
	MsgIdCloseClientConnection = -2
)

type ISession interface {
	GetMid() int64
	GetIndex() uint32
	GetCode() int64
	Response(request interface{}) error
	ResponseCode(code int64) error
	Forward(toPid *Pid) error
	Push(request interface{}) error
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
	message, err := a.buildMessage(MsgIdPushMessageToClient, request)
	if err != nil {
		return err
	}
	return a.send(message)
}

func (a *WrapSession) ResponseCode(code int64) error {
	a.Session.Code = code
	message, err := a.buildMessage(MsgIdPushMessageToClient, []byte{})
	if err != nil {
		return err
	}
	return a.send(message)
}

func (a *WrapSession) Forward(toPid *Pid) error {
	msg := a.ctx.Message()
	if msg == nil {
		return errors.New("no message to forward")
	}
	msg.To = toPid
	msg.From = a.ctx.ID()
	return a.send(msg)
}

func (a *WrapSession) Push(request interface{}) error {
	message, err := a.buildMessage(MsgIdPushMessageToClient, request)
	if err != nil {
		return err
	}
	return a.send(message)
}

// sendToSession 发送消息到会话，如果是本地则直接调用，否则通过系统发送
func (a *WrapSession) send(message *Message) error {
	message.Session = a.Session
	if a.GetAgent() == a.ctx.ID() {
		return a.ctx.InvokerMessage(message)
	}
	return a.ctx.System().Send(message)
}

func (a *WrapSession) Close() error {
	message, err := a.buildMessage(MsgIdCloseClientConnection, []byte{})
	if err != nil {
		return err
	}
	return a.send(message)
}

func (a *WrapSession) buildMessage(msgId int64, request interface{}) (*Message, error) {
	return BuildMessage(a.ctx.GetSerializer(), a.ctx.ID(), a.GetAgent(), msgId, request)
}

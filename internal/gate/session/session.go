package session

import (
	"github.com/dzm2020/gas/internal/iface"

	"github.com/duke-git/lancet/v2/convertor"
)

const (
	PushMessageToClientMethod   = "Push"
	CloseClientConnectionMethod = "Shutdown"
	SetValueMethod              = "SetValue"
)

func New(entityId int64, pid *iface.Pid) *Session {
	return &Session{
		Session: &iface.Session{
			EntityId: entityId,
			Agent:    pid,
		},
	}
}

func NewWithSession(session *iface.Session) *Session {
	return &Session{
		Session: session,
	}
}

type Session struct {
	*iface.Session
	ctx iface.IContext
}

func (a *Session) SetPid(pid *iface.Pid) {
	a.Agent = pid
}

func (a *Session) SetEntity(entityId int64) {
	a.EntityId = entityId
}

func (a *Session) SetContext(ctx iface.IContext) {
	a.ctx = ctx
}
func (a *Session) Meta() *iface.Session {
	return a.Session
}
func (a *Session) Response(request interface{}) error {
	node := a.ctx.Node()
	bin, err := node.Marshal(request)
	if err != nil {
		return err
	}
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

func (a *Session) SetValue(key, value string) error {
	message := iface.NewActorMessage(a.ctx.ID(), a.GetAgent(), SetValueMethod, nil)
	message.Session = convertor.DeepClone(a.Session)
	if a.Session.Values == nil {
		a.Session.Values = make(map[string]string)
	}
	message.Session.Values[key] = value
	return a.send(message)
}

func (a *Session) Push(cmd, act uint16, request interface{}) error {
	node := a.ctx.Node()
	bin, err := node.Marshal(request)
	if err != nil {
		return err
	}
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
		system := a.ctx.Node().System()
		return system.Send(message)
	}
}

func (a *Session) Close() error {
	message := iface.NewActorMessage(a.ctx.ID(), a.GetAgent(), CloseClientConnectionMethod, nil)
	return a.send(message)
}

package session

import (
	"errors"

	"github.com/dzm2020/gas/internal/iface"
)

const (
	PushMessageToClientMethod   = "Push"
	CloseClientConnectionMethod = "Shutdown"
	SetValueMethod              = "SetValue"
)

var (
	errTransportIsNil = errors.New("transport is nil")
)

type ITransport interface {
	Send(session iface.ISession, route string, payload interface{}) error
}

func New(session *iface.Session, transport ITransport) *Session {
	return &Session{
		Session:   session,
		transport: transport,
	}
}

type Session struct {
	*iface.Session            // 原始数据
	transport      ITransport // 处理session写入
}

func (a *Session) Raw() *iface.Session {
	return a.Session
}
func (a *Session) check() error {
	if a.transport == nil {
		return errTransportIsNil
	}
	return nil
}
func (a *Session) Response(request interface{}) error {
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Send(a, PushMessageToClientMethod, request)
}

func (a *Session) ResponseCode(code int64) error {
	a.Code = code
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Send(a, PushMessageToClientMethod, nil)
}

func (a *Session) SetValue(key, value string) error {
	if a.Values == nil {
		a.Values = make(map[string]string)
	}
	a.Values[key] = value
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Send(a, SetValueMethod, nil)
}

func (a *Session) Push(cmd, act uint16, request interface{}) error {
	a.Cmd = uint32(cmd)
	a.Act = uint32(act)
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Send(a, PushMessageToClientMethod, request)
}

func (a *Session) Close() error {
	if err := a.check(); err != nil {
		return err
	}
	return a.transport.Send(a, CloseClientConnectionMethod, nil)
}

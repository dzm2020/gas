package gate

import (
	"gas/internal/protocol"
	"gas/pkg/actor"
	"gas/pkg/network"
	"sync/atomic"
)

var sessionId atomic.Uint64

func NewSession(gate *Gate) *Session {
	session := &Session{
		ID:   sessionId.Add(1),
		gate: gate,
	}
	gate.mgr.Add(session)
	return session
}

type Session struct {
	ID uint64
	network.IEntity
	gate  *Gate
	actor actor.IProcess
	msg   *protocol.Message
}

func (m *Session) GetID() uint64 {
	return m.ID
}
func (m *Session) OnConnect(entity network.IEntity) error {
	m.IEntity = entity
	m.actor = actor.Spawn(m.gate.producer, nil)
	return m.actor.Post("OnSessionOpen", m)
}

func (m *Session) OnTraffic(msg *protocol.Message) error {
	m.msg = msg
	return m.gate.Router().Handle(m, msg)
}

func (m *Session) GetActor() actor.IProcess {
	return m.actor
}

func (m *Session) OnClose(err error) error {
	m.gate.mgr.Remove(m.GetID())
	return m.actor.Post("OnSessionClose", m)
}

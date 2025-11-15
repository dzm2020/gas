package gate

import (
	"fmt"

	"gas/internal/gate/codec"
	"gas/pkg/actor"
	"gas/pkg/network"
	"gas/pkg/utils/buffer"
)

type ISession interface {
	network.IEntity
	GetAgent() actor.IProcess
}

func NewSession(gate *Gate, entity network.IEntity) *Session {
	session := &Session{
		gate:    gate,
		IEntity: entity,
		buffer:  buffer.New(int(gate.opts.ReadBufferSize)),
		codec:   gate.opts.Codec,
	}
	return session
}

type Session struct {
	network.IEntity
	gate   *Gate
	agent  actor.IProcess
	buffer buffer.IBuffer
	codec  codec.ICodec
}

func (m *Session) OnConnect() error {
	producer := m.gate.producer
	if producer == nil {
		return fmt.Errorf("gate: agent producer is nil")
	}

	m.agent = actor.Spawn(func() actor.IActor {
		return producer()
	})

	return m.agent.PushTask(func(ctx actor.IContext) error {
		agent := ctx.Actor().(IAgent)
		return agent.OnSessionOpen(ctx, m)
	})
}

func (m *Session) OnTraffic() error {
	//  写入缓冲区BUFFER
	if _, err := m.buffer.WriteReader(m.IEntity); err != nil {
		return err
	}
	//  解包
	msgList, readN := m.codec.Decode(m.buffer.Bytes())
	for _, msg := range msgList {
		if err := m.gate.Router().Handle(m, msg); err != nil {
			return err
		}
	}
	//  移动缓冲区指针
	return m.buffer.Skip(readN)
}

func (m *Session) OnClose(err error) error {
	return m.agent.PushTask(func(ctx actor.IContext) (wrong error) {
		agent := ctx.Actor().(IAgent)
		wrong = agent.OnSessionClose(ctx, m)
		m.gate.mgr.Remove(m.ID())
		return
	})
}

func (m *Session) GetAgent() actor.IProcess {
	return m.agent
}

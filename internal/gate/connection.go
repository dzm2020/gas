package gate

import (
	"fmt"
	"gas/internal/gate/codec"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/pkg/network"
	"gas/pkg/utils/buffer"
)

func NewConnection(gate *Gate, entity network.IEntity) *Connection {
	connection := &Connection{
		gate:    gate,
		IEntity: entity,
		buffer:  buffer.New(int(gate.opts.ReadBufferSize)),
		codec:   gate.opts.Codec,
	}
	return connection
}

type Connection struct {
	network.IEntity
	gate   *Gate
	agent  iface.IProcess
	buffer buffer.IBuffer
	codec  codec.ICodec
}

func (m *Connection) OnConnect() error {
	factory := m.gate.factory
	if factory == nil {
		return fmt.Errorf("gate: agent factory is nil")
	}

	_, m.agent = m.gate.actorSystem.Spawn(factory())

	return m.agent.PushTask(func(ctx iface.IContext) error {
		agent := ctx.Actor().(IAgent)
		return agent.OnConnectionOpen(ctx, m)
	})
}

func (m *Connection) OnTraffic() error {
	//  写入缓冲区BUFFER
	if _, err := m.buffer.WriteReader(m.IEntity); err != nil {
		return err
	}
	//  解包
	msgList, readN := m.codec.Decode(m.buffer.Bytes())
	for _, msg := range msgList {
		if err := m.agent.PushMessage(msg); err != nil {
			return err
		}
	}
	//  移动缓冲区指针
	return m.buffer.Skip(readN)
}

func (m *Connection) OnClose(err error) error {
	return m.agent.PushTask(func(ctx iface.IContext) (wrong error) {
		agent := ctx.Actor().(IAgent)
		wrong = agent.OnConnectionClose(ctx)
		return
	})
}

func (m *Connection) Send(msg *protocol.Message) error {
	bin := m.codec.Encode(msg)
	return m.IEntity.Write(bin)
}

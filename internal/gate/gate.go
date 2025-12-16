package gate

import (
	"context"
	"gas/internal/errs"
	"gas/internal/gate/codec"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/pkg/network"
)

type Gate struct {
	network.EmptyHandler
	Address string
	Options []network.Option
	Factory Factory
	server  network.IServer
}

func (m *Gate) Name() string {
	return "gate"
}

func (m *Gate) Start(ctx context.Context) error {
	var err error
	m.server, err = network.NewServer(m.Address, append(m.Options, network.WithHandler(m), network.WithCodec(codec.New()))...)
	if err != nil {
		return err
	}
	if err = m.server.Start(); err != nil {
		return err
	}
	return nil
}

func (m *Gate) OnConnect(entity network.IConnection) (err error) {
	system := iface.GetNode().System()

	actor := m.Factory()
	pid := system.Spawn(actor)

	entity.SetContext(pid)

	return system.PushTask(pid, func(ctx iface.IContext) error {
		_agent := ctx.Actor().(IAgent)
		session := iface.NewSessionWithPid(ctx, pid)
		return _agent.OnNetworkOpen(ctx, session)
	})
}

func (m *Gate) OnMessage(entity network.IConnection, msg interface{}) error {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return errs.ErrAgentNoBindConnection
	}

	system := iface.GetNode().System()

	//  将网关消息转为内容消息
	clientMessage, ok := msg.(*protocol.Message)
	if !ok {
		return errs.ErrInvalidMessageType
	}

	actorMsg := m.clientToActorMessage(pid, entity, clientMessage)

	return system.Send(actorMsg)
}

func (m *Gate) OnClose(entity network.IConnection, wrong error) error {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return errs.ErrAgentNoBindConnection
	}

	system := iface.GetNode().System()
	return system.PushTask(pid, func(ctx iface.IContext) error {
		_agent := ctx.Actor().(IAgent)
		session := iface.NewSessionWithPid(ctx, pid)
		return _agent.OnNetworkClose(ctx, session)
	})
}

func (m *Gate) clientToActorMessage(agent *iface.Pid, entity network.IConnection, clientMessage *protocol.Message) *iface.ActorMessage {
	message := iface.NewActorMessage(agent, agent, "OnNetworkMessage", clientMessage.Data)
	message.Session = &iface.Session{
		Agent:    agent,
		Mid:      int64(clientMessage.ID()),
		Index:    clientMessage.Index,
		EntityId: entity.ID(),
	}
	return message
}

func (m *Gate) Stop(ctx context.Context) error {
	if m.server != nil {
		_ = m.server.Stop()
	}
	return nil
}

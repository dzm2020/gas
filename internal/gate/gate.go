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
	node    iface.INode
}

func (m *Gate) Name() string {
	return "gate"
}
func (m *Gate) Start(ctx context.Context, node iface.INode) error {
	m.node = node
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
	factory := m.Factory
	if factory == nil {
		return errs.ErrAgentFactoryNil
	}
	system := m.node.GetActorSystem()
	pid := system.Spawn(factory())

	entity.SetContext(pid)

	return system.PushTask(pid, func(ctx iface.IContext) error {
		_agent := ctx.Actor().(IAgent)
		session := iface.NewSession(ctx, &iface.Session{Agent: pid})
		return _agent.OnNetworkOpen(ctx, session)
	})
}

func (m *Gate) OnMessage(entity network.IConnection, msg interface{}) error {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return errs.ErrAgentNoBindConnection
	}

	system := m.node.GetActorSystem()

	//  将网关消息转为内容消息
	clientMessage, ok := msg.(*protocol.Message)
	if !ok {
		return errs.ErrInvalidMessageType
	}
	session := &iface.Session{
		Agent:    pid,
		Mid:      int64(clientMessage.ID()),
		Index:    clientMessage.Index,
		EntityId: entity.ID(),
	}

	message := iface.NewActorMessage(pid, pid, "OnNetworkMessage")
	message.Session = session
	message.Data = clientMessage.Data
	return system.Send(message)
}

func (m *Gate) OnClose(entity network.IConnection, wrong error) error {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return errs.ErrAgentNoBindConnection
	}

	system := m.node.GetActorSystem()
	return system.PushTask(pid, func(ctx iface.IContext) (wrong error) {
		_agent := ctx.Actor().(IAgent)
		session := iface.NewSession(ctx, &iface.Session{Agent: pid})
		wrong = _agent.OnNetworkClose(ctx, session)
		return
	})
}

func (m *Gate) Stop(ctx context.Context) error {
	if m.server != nil {
		_ = m.server.Stop()
	}
	return nil
}

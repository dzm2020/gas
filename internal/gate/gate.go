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

func (g *Gate) Name() string {
	return "gate"
}

func (g *Gate) Start(ctx context.Context) error {
	var err error
	g.server, err = network.NewServer(g.Address, append(g.Options, network.WithHandler(g), network.WithCodec(codec.New()))...)
	if err != nil {
		return err
	}
	if err = g.server.Start(); err != nil {
		return err
	}
	return nil
}

func (g *Gate) OnConnect(entity network.IConnection) (err error) {
	node := iface.GetNode()

	system := node.System()

	actor := g.Factory()
	pid := system.Spawn(actor)

	entity.SetContext(pid)

	return system.SubmitTask(pid, func(ctx iface.IContext) error {
		agent, _ := ctx.Actor().(IAgent)
		session := iface.NewSessionWithPid(ctx, pid)
		return agent.OnNetworkOpen(ctx, session)
	})
}

func (g *Gate) OnMessage(entity network.IConnection, msg interface{}) error {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return errs.ErrAgentNoBindConnection
	}

	//  将网关消息转为内容消息
	clientMessage, ok := msg.(*protocol.Message)
	if !ok {
		return errs.ErrInvalidMessageType
	}

	node := iface.GetNode()
	system := node.System()

	actorMsg := g.clientToActorMessage(pid, entity, clientMessage)

	return system.Send(actorMsg)
}

func (g *Gate) OnClose(entity network.IConnection, wrong error) error {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return errs.ErrAgentNoBindConnection
	}

	node := iface.GetNode()
	system := node.System()

	return system.SubmitTask(pid, func(ctx iface.IContext) error {
		agent := ctx.Actor().(IAgent)
		session := iface.NewSessionWithPid(ctx, pid)
		return agent.OnNetworkClose(ctx, session)
	})
}

func (g *Gate) clientToActorMessage(agent *iface.Pid, entity network.IConnection, clientMessage *protocol.Message) *iface.ActorMessage {
	message := iface.NewActorMessage(agent, agent, "OnNetworkMessage", clientMessage.Data)
	message.Session = &iface.Session{
		Agent:    agent,
		Mid:      int64(clientMessage.ID()),
		Index:    clientMessage.Index,
		EntityId: entity.ID(),
	}
	return message
}

func (g *Gate) Stop(ctx context.Context) error {
	if g.server != nil {
		_ = g.server.Stop()
	}
	return nil
}

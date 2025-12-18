package gate

import (
	"context"
	"gas/internal/errs"
	"gas/internal/gate/codec"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/internal/session"
	"gas/pkg/network"

	"github.com/duke-git/lancet/v2/convertor"
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

	s := &session.Session{
		Session: &iface.Session{
			Agent:    pid,
			EntityId: entity.ID(),
		},
	}

	entity.SetContext(s)

	ss := convertor.DeepClone(s)

	return system.SubmitTask(s.GetAgent(), func(ctx iface.IContext) error {
		agent, _ := ctx.Actor().(IAgent)
		ss.SetContext(ctx)
		return agent.OnNetworkOpen(ctx, ss)
	})
}

func (g *Gate) OnMessage(entity network.IConnection, msg interface{}) error {
	s, _ := entity.Context().(*session.Session)
	if s == nil {
		return errs.ErrNoBindConnection
	}

	ss := convertor.DeepClone(s)

	//  将网关消息转为内容消息
	clientMessage, ok := msg.(*protocol.Message)
	if !ok {
		return errs.ErrInvalidMessageType
	}

	node := iface.GetNode()
	system := node.System()

	actorMsg := g.convertMessage(ss, clientMessage)

	return system.Send(actorMsg)
}

func (g *Gate) OnClose(entity network.IConnection, wrong error) error {
	s, _ := entity.Context().(*session.Session)
	if s == nil {
		return errs.ErrNoBindConnection
	}

	node := iface.GetNode()
	system := node.System()

	ss := convertor.DeepClone(s)
	return system.SubmitTask(ss.GetAgent(), func(ctx iface.IContext) error {
		agent := ctx.Actor().(IAgent)
		ss.SetContext(ctx)
		return agent.OnNetworkClose(ctx, s)
	})
}

func (g *Gate) convertMessage(s *session.Session, msg *protocol.Message) *iface.ActorMessage {
	agent := s.GetAgent()
	message := iface.NewActorMessage(agent, agent, "OnNetworkMessage", msg.Data)
	message.Session = &iface.Session{
		Cmd:   uint32(msg.Cmd),
		Act:   uint32(msg.Act),
		Index: msg.Index,
	}
	return message
}

func (g *Gate) Stop(ctx context.Context) error {
	if g.server != nil {
		_ = g.server.Stop()
	}
	return nil
}

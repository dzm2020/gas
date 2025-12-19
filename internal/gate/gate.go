package gate

import (
	"context"
	"errors"
	"gas/internal/gate/codec"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/internal/session"
	"gas/pkg/network"

	"github.com/duke-git/lancet/v2/convertor"
)

var (
	ErrNoBindConnection   = errors.New("no bind connection")
	ErrInvalidMessageType = errors.New("gate: invalid message type")
)

type Gate struct {
	network.EmptyHandler
	node    iface.INode
	Address string
	Options []network.Option
	Factory Factory
	server  network.IServer
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
	system := g.node.System()

	pid := system.Spawn(g.Factory())

	s := session.NewWithPid(pid, entity.ID())

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
		return ErrNoBindConnection
	}

	ss := convertor.DeepClone(s)

	//  将网关消息转为内容消息
	clientMessage, ok := msg.(*protocol.Message)
	if !ok {
		return ErrInvalidMessageType
	}

	actorMsg := g.convertMessage(ss, clientMessage)

	return g.node.System().Send(actorMsg)
}

func (g *Gate) OnClose(entity network.IConnection, wrong error) error {
	s, _ := entity.Context().(*session.Session)
	if s == nil {
		return ErrNoBindConnection
	}

	node := g.node
	system := g.node.System()

	ss := convertor.DeepClone(s)
	return g.node.System().SubmitTask(ss.GetAgent(), func(ctx iface.IContext) error {
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

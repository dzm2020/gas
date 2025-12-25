package gate

import (
	"context"
	"errors"
	"gas/internal/gate/codec"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/internal/session"
	"gas/pkg/glog"
	"gas/pkg/network"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type Gate struct {
	network.EmptyHandler
	node    iface.INode
	Address string
	Options []network.Option
	Factory Factory
	server  network.IServer
	maxConn int64
	count   atomic.Int64
}

func (g *Gate) Start(ctx context.Context) (err error) {
	options := append(g.Options, network.WithHandler(g), network.WithCodec(codec.New()))
	g.server, err = network.NewServer(g.Address, options...)
	if err != nil {
		return
	}
	return g.server.Start()
}

func (g *Gate) getSession(entity network.IConnection) *session.Session {
	s, ok := entity.Context().(*session.Session)
	if !ok || s == nil {
		//  创建agent
		system := g.node.System()
		pid := system.Spawn(g.Factory())
		//  绑定
		s = session.New()
		s.SetEntity(entity.ID())
		s.SetPid(pid)

		entity.SetContext(s)

		glog.Debug("网关:创建session", zap.Int64("entityId", entity.ID()), zap.Any("pid", pid))
	}
	return convertor.DeepClone(s)
}

func (g *Gate) OnConnect(entity network.IConnection) error {
	if g.count.Load() > g.maxConn {
		return errors.New("too many connections")
	}
	g.count.Add(1)

	s := g.getSession(entity)
	system := g.node.System()

	message := g.makeActorMessage(s, "OnConnectionOpen", nil)

	return system.Send(message)
}

func (g *Gate) OnMessage(entity network.IConnection, clientMsg interface{}) error {
	system := g.node.System()
	s := g.getSession(entity)

	msg, _ := clientMsg.(*protocol.Message)

	g.formatSession(s, msg)

	message := g.makeActorMessage(s, "OnConnectionMessage", msg.Data)

	return system.Send(message)
}

func (g *Gate) OnClose(entity network.IConnection, wrong error) error {
	g.count.Add(-1)

	s := g.getSession(entity)
	system := g.node.System()

	message := g.makeActorMessage(s, "OnConnectionClose", nil)

	return system.Send(message)
}

func (g *Gate) makeActorMessage(session *session.Session, method string, data []byte) *iface.ActorMessage {
	agent := session.GetAgent()
	message := iface.NewActorMessage(agent, agent, method, data)
	message.Session = session.Session
	return message
}

func (g *Gate) formatSession(s *session.Session, msg *protocol.Message) {
	s.Cmd = uint32(msg.Cmd)
	s.Act = uint32(msg.Act)
	s.Index = msg.Index
	return
}

func (g *Gate) Stop(ctx context.Context) error {
	if g.server == nil {
		return nil
	}
	return g.server.Shutdown()
}

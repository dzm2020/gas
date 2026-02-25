package gate

import (
	"context"
	"errors"

	"github.com/dzm2020/gas/internal/gate/codec"
	"github.com/dzm2020/gas/internal/gate/protocol"
	"github.com/dzm2020/gas/internal/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/network"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type Gate struct {
	network.EmptyHandler
	Address string
	Options []network.Option
	Factory Factory
	server  network.IServer
	MaxConn int64
	count   atomic.Int64
	system  iface.ISystem
}

func (g *Gate) Start(ctx context.Context, system iface.ISystem) (err error) {
	g.system = system
	g.server, err = network.NewServer(g, g.Address, g.Options...)
	if err != nil {
		return
	}
	return g.server.Start()
}

func (g *Gate) getOrCreateSession(entity network.IConnection) *session.Session {
	s, ok := entity.Context().(*session.Session)
	if !ok || s == nil {
		//  创建agent
		agent := g.Factory()
		system := g.system
		pid := system.Spawn(agent)
		//  绑定
		s = session.New(entity.ID(), pid)
		entity.SetContext(s)

		glog.Debug("网关:创建Agent", zap.Int64("entityId", entity.ID()), zap.Any("pid", pid))
	}
	return s
}

// submitToAgent 封装「取 session → 向 agent 投递任务」的公共逻辑
func (g *Gate) submitToAgent(entity network.IConnection, fn func(ctx iface.IContext, agent IAgent, s *session.Session) error) error {
	s := g.getOrCreateSession(entity)
	system := g.system

	return system.SubmitTask(s.GetAgent(), func(ctx iface.IContext) error {
		agent := ctx.Actor().(IAgent)
		return fn(ctx, agent, s)
	})
}

func (g *Gate) OnConnect(entity network.IConnection) error {
	if g.count.Load() > g.MaxConn {
		return errors.New("too many connections")
	}
	g.count.Add(1)
	return g.submitToAgent(entity, func(ctx iface.IContext, agent IAgent, s *session.Session) error {
		return agent.OnOpen(ctx, s)
	})
}

func (g *Gate) OnMessage(entity network.IConnection, data []byte) (n int, err error) {
	var msg *protocol.Message
	var processN int
	for len(data) > 0 {
		msg, processN, err = codec.Decode(data)
		if err != nil {
			return
		}
		if processN == 0 {
			return
		}
		n += processN
		data = data[processN:]

		err = g.submitToAgent(entity, func(ctx iface.IContext, agent IAgent, s *session.Session) error {
			ss := convertor.DeepClone(s)
			ss.Cmd = uint32(msg.Cmd)
			ss.Act = uint32(msg.Act)
			ss.Index = msg.Index
			return agent.OnData(ctx, ss, msg.Data)
		})
		if err != nil {
			return
		}
	}
	return
}

func (g *Gate) OnClose(entity network.IConnection, wrong error) {
	g.count.Add(-1)
	_ = g.submitToAgent(entity, func(ctx iface.IContext, agent IAgent, s *session.Session) error {
		return agent.OnClose(ctx, s)
	})
}

func (g *Gate) Stop(ctx context.Context) error {
	if g.server == nil {
		return nil
	}
	g.server.Shutdown(ctx)
	return nil
}

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
	Address     string
	Options     []network.Option
	Factory     Factory
	Middlewares []Middleware // Decode 后 / Encode 前 按顺序执行
	server      network.IServer
	MaxConn     int64
	count       atomic.Int64
	system      iface.ISystem
}

func (g *Gate) Start(ctx context.Context, system iface.ISystem) (err error) {
	g.system = system
	g.server, err = network.NewServer(g, g.Address, g.Options...)
	if err != nil {
		return
	}
	return g.server.Start()
}

func (g *Gate) OnConnect(entity network.IConnection) error {
	if g.count.Load() > g.MaxConn {
		return errors.New("too many connections")
	}
	g.count.Add(1)

	//  创建agent
	agent := g.Factory()
	agent.SetMiddleware(g.Middlewares)
	system := g.system
	pid := system.Spawn(agent)
	//  绑定
	s := session.New(entity.ID(), pid)
	entity.SetContext(s)

	glog.Debug("网关:创建Agent", zap.Int64("entityId", entity.ID()), zap.Any("pid", pid))
	return nil
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

		//  提交到agent actor处理
		s, ok := entity.Context().(*session.Session)
		if !ok || s == nil || s.GetAgent() == nil {
			return n, nil
		}
		err = g.system.SubmitTask(s.GetAgent(), func(ctx iface.IContext) error {
			return g.onData(ctx, msg, s)
		})
		if err != nil {
			return
		}
	}
	return
}

func (g *Gate) onData(ctx iface.IContext, msg *protocol.Message, s *session.Session) error {
	var err error
	agent := ctx.Actor().(IAgent)
	if len(g.Middlewares) > 0 {
		msg, err = RunAfterDecode(g.Middlewares, msg)
		if err != nil {
			return err
		}
		if msg == nil {
			return nil
		}
	}
	ss := convertor.DeepClone(s)
	ss.Cmd = uint32(msg.Cmd)
	ss.Act = uint32(msg.Act)
	ss.Index = msg.Index
	return agent.OnData(ctx, ss, msg.Data)
}

func (g *Gate) OnClose(entity network.IConnection, wrong error) {
	g.count.Add(-1)
	s, ok := entity.Context().(*session.Session)
	if !ok || s == nil || s.GetAgent() == nil {
		return
	}
	g.system.ShutdownProcess(s.GetAgent())
}

func (g *Gate) Stop(ctx context.Context) error {
	if g.server == nil {
		return nil
	}
	g.server.Shutdown(ctx)
	return nil
}

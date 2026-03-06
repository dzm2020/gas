package gate

import (
	"context"
	"errors"

	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/component/gate/session"
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
	agent.AppendMiddleware(g.Middlewares...)
	system := g.system
	pid := system.Spawn(agent)
	//  绑定
	s := session.New(entity.ID())
	entity.SetContext(s)

	glog.Debug("网关:创建Agent", zap.Int64("entityId", entity.ID()), zap.Any("pid", pid))
	return nil
}

func (g *Gate) OnMessage(entity network.IConnection, data []byte) (n int, err error) {
	var msg *protocol.Message
	var processN int
	for len(data) > 0 {
		//  解包
		msg, processN, err = codec.Decode(data)
		if err != nil {
			return
		}
		if processN == 0 {
			return
		}
		n += processN
		data = data[processN:]

		//  运行中间件
		if len(g.Middlewares) > 0 {
			msg, err = RunAfterDecode(g.Middlewares, msg)
			if err != nil {
				return
			}
			if msg == nil {
				return
			}
		}
		//  提交到agent actor处理
		actorMsg := g.convertMessage(msg, g.getSession(entity))
		if err = g.system.Send(actorMsg); err != nil {
			return
		}
	}
	return
}

func (g *Gate) convertMessage(clientMsg *protocol.Message, s *session.Session) *iface.ActorMessage {
	actorMsg := iface.NewActorMessage(nil, s.GetAgent(), "OnData", clientMsg.Data)
	if s != nil {
		ss := convertor.DeepClone(s)
		ss.SetCmd(clientMsg.Cmd)
		ss.SetAct(clientMsg.Act)
		ss.SetIndex(clientMsg.Index)
		actorMsg.Session = ss.Raw()
	}
	return actorMsg
}

func (g *Gate) OnClose(entity network.IConnection, wrong error) {
	g.count.Add(-1)
	s := g.getSession(entity)
	if s == nil {
		return
	}
	_ = g.system.ShutdownProcess(s.GetAgent())
}

func (g *Gate) getSession(entity network.IConnection) *session.Session {
	s, _ := entity.Context().(*session.Session)
	return s
}

func (g *Gate) Stop(ctx context.Context) error {
	if g.server == nil {
		return nil
	}
	g.server.Shutdown(ctx)
	return nil
}

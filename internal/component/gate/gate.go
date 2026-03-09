package gate

import (
	"context"
	"errors"

	"github.com/dzm2020/gas/internal/component/gate/agent"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/network"

	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type Gate struct {
	network.EmptyHandler
	Address string
	Options []network.Option
	Factory agent.Factory
	MaxConn int64
	server  network.IServer
	count   atomic.Int64
	system  iface.ISystem
}

func (g *Gate) Start(ctx context.Context, system iface.ISystem) (err error) {
	g.system = system
	g.server, err = network.NewServer(g, g.Address, g.Options...)
	if err != nil {
		return
	}
	system.SetSessionFactory(&session.Factory{})
	return g.server.Start()
}

func (g *Gate) OnConnect(entity network.IConnection) error {
	if g.count.Load() > g.MaxConn {
		return errors.New("too many connections")
	}
	g.count.Add(1)
	//  创建agent
	pid := g.system.Spawn(agent.New(entity, g.Factory()))
	//  绑定到entity
	entity.SetContext(pid)
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
		//  处理消息
		if err = g.process(entity, msg); err != nil {
			return
		}
	}
	return
}

func (g *Gate) process(entity network.IConnection, msg *protocol.Message) (err error) {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return
	}
	//  提交到agent actor处理
	return g.system.SubmitTask(pid, func(ctx iface.IContext) error {
		a := ctx.Actor().(*agent.Agent)
		return a.OnData(ctx, msg)
	})
}

func (g *Gate) OnClose(entity network.IConnection, wrong error) {
	g.count.Add(-1)
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return
	}
	_ = g.system.ShutdownProcess(pid)
}

func (g *Gate) Stop(ctx context.Context) error {
	if g.server == nil {
		return nil
	}
	g.server.Shutdown(ctx)
	return nil
}

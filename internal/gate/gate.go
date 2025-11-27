package gate

import (
	"context"
	"fmt"
	"gas/internal/gate/codec"
	"gas/internal/iface"
	"gas/pkg/lib/glog"
	"gas/pkg/lib/workers"
	"gas/pkg/network"

	"go.uber.org/zap"
)

func New(factory Factory, opts ...Option) *Gate {
	gate := &Gate{
		factory: factory,
		opts:    loadOptions(opts...),
	}
	return gate
}

type Gate struct {
	network.EmptyHandler
	opts    *Options
	factory Factory
	server  network.IServer
	ctx     *workers.WaitContext
}

func (m *Gate) Run(ctx context.Context) {
	var err error
	m.server, err = network.NewServer(m.opts.ProtoAddr, network.WithHandler(m), network.WithCodec(codec.New()))

	if err != nil {
		glog.Error("gate run err", zap.Error(err))
	}

	m.ctx.Add(1)
	workers.Go(func(ctx *workers.WaitContext) {
		defer m.ctx.Done()
		if err = m.server.Start(m.ctx); err != nil {
			glog.Errorf("gate run listening on %s err:%v", m.server.Addr().String(), err)
		}
	})
	glog.Infof("gate run listening on %s", m.server.Addr().String())
}

func (m *Gate) OnConnect(entity network.IConnection) (err error) {
	if network.ConnectionCount() > int64(m.opts.MaxConn) {
		return fmt.Errorf("maximum number of connections exceeded")
	}

	factory := m.factory
	if factory == nil {
		return fmt.Errorf("gate: agent factory is nil")
	}
	//  创建agent
	agent := factory()
	//  绑定
	entity.SetContext(agent)
	//  执行初始化
	return agent.PushTask(func(ctx iface.IContext) error {
		_agent := ctx.Actor().(IAgent)
		return _agent.OnConnect(ctx, entity)
	})
}

func (m *Gate) OnMessage(entity network.IConnection, msg interface{}) error {
	agent, _ := entity.Context().(iface.IProcess)
	if agent == nil {
		return fmt.Errorf("no bind connection")
	}
	return agent.PushMessage(msg)
}

func (m *Gate) OnClose(entity network.IConnection, wrong error) {
	agent, _ := entity.Context().(iface.IProcess)
	if agent == nil {
		return
	}
	_ = agent.PushTask(func(ctx iface.IContext) (wrong error) {
		_agent := ctx.Actor().(IAgent)
		wrong = _agent.OnClose(ctx)
		return
	})
}

func (m *Gate) GracefulStop() {
	if m.server != nil {
		_ = m.server.Stop()
	}
	if err := m.ctx.WaitWithTimeout(m.opts.GracePeriod); err != nil {
		glog.Error("gate graceful stop err", zap.Error(err))
	}
}

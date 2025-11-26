package gate

import (
	"fmt"
	"gas/internal/gate/codec"
	"gas/internal/iface"
	"gas/pkg/network"
	"gas/pkg/utils/glog"
	"gas/pkg/utils/workers"
	"time"

	"go.uber.org/zap"
)

func New(node iface.INode, factory Factory, opts ...Option) *Gate {
	gate := &Gate{
		factory:     factory,
		opts:        loadOptions(opts...),
		actorSystem: node.GetActorSystem(),
	}
	return gate
}

type Gate struct {
	network.EmptyHandler
	opts        *Options
	factory     Factory
	server      network.IServer
	actorSystem iface.ISystem
}

func (m *Gate) Run() {
	var err error
	m.server, err = network.NewServer(m.opts.ProtoAddr, network.WithHandler(m), network.WithCodec(codec.New()))
	if err != nil {
		glog.Error("gate run err", zap.Error(err))
	}
	workers.Submit(func() {
		if err = m.server.Start(); err != nil {
			glog.Errorf("gate run listening on %s err:%v", m.server.Addr().String(), err)
		}
	}, nil)

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
	_, agent := m.actorSystem.Spawn(factory())
	//  绑定
	entity.SetContext(agent)
	//  执行初始化
	return agent.PushTask(func(ctx iface.IContext) error {
		_agent := ctx.Actor().(IAgent)
		return _agent.OnConnectionOpen(ctx, entity)
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
		wrong = _agent.OnConnectionClose(ctx)
		return
	})
}

func (m *Gate) GracefulStop() {
	if m.server == nil {
		return
	}

	_ = m.server.Stop()

	const checkInterval = 50 * time.Millisecond

	gracePeriod := m.opts.GracePeriod
	if gracePeriod <= 0 {
		gracePeriod = 5 * time.Second
	}

	timeout := time.After(gracePeriod)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		if network.ConnectionCount() == 0 {
			return
		}

		select {
		case <-ticker.C:
		case <-timeout:
			return
		}
	}
}

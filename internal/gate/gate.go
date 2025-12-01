package gate

import (
	"errors"
	"gas/internal/gate/codec"
	"gas/internal/iface"
	"gas/pkg/glog"
	"gas/pkg/network"
)

var (
	ErrAgentFactoryNil       = errors.New("gate: agent factory is nil")
	ErrAgentNoBindConnection = errors.New("no bind connection")
)

func New(address string, factory Factory, opts ...network.Option) *Gate {
	gate := &Gate{
		opts:    opts,
		address: address,
		factory: factory,
	}
	return gate
}

type Gate struct {
	network.EmptyHandler
	address string
	opts    []network.Option
	factory Factory
	server  network.IServer
}

func (m *Gate) Run() error {
	var err error
	m.server, err = network.NewServer(m.address, append(m.opts, network.WithHandler(m), network.WithCodec(codec.New()))...)
	if err != nil {
		return err
	}
	if err = m.server.Start(); err != nil {
		return err
	}
	glog.Infof("gate run listening on %s", m.server.Addr())
	return nil
}

func (m *Gate) OnConnect(entity network.IConnection) (err error) {
	factory := m.factory
	if factory == nil {
		return ErrAgentFactoryNil
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
		return ErrAgentNoBindConnection
	}
	return agent.PushMessage(msg)
}

func (m *Gate) OnClose(entity network.IConnection, wrong error) error {
	agent, _ := entity.Context().(iface.IProcess)
	if agent == nil {
		return ErrAgentNoBindConnection
	}
	return agent.PushTask(func(ctx iface.IContext) (wrong error) {
		_agent := ctx.Actor().(IAgent)
		wrong = _agent.OnClose(ctx)
		return
	})
}

func (m *Gate) Stop() error {
	if m.server != nil {
		_ = m.server.Stop()
	}
	return nil
}

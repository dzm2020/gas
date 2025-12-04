package gate

import (
	"context"
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

type Gate struct {
	network.EmptyHandler
	Address string
	Options []network.Option
	Factory Factory
	server  network.IServer
	node    iface.INode
}

func (m *Gate) Name() string {
	return "gate"
}
func (m *Gate) Start(ctx context.Context, node iface.INode) error {
	m.node = node
	var err error
	m.server, err = network.NewServer(m.Address, append(m.Options, network.WithHandler(m), network.WithCodec(codec.New()))...)
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
	factory := m.Factory
	if factory == nil {
		return ErrAgentFactoryNil
	}

	system := m.node.GetActorSystem()

	pid := system.Spawn(factory())

	//  绑定
	entity.SetContext(pid)
	//  执行初始化
	return system.PushTask(pid, func(ctx iface.IContext) error {
		_agent := ctx.Actor().(IAgent)
		return _agent.OnConnect(ctx, entity)
	})
}

func (m *Gate) OnMessage(entity network.IConnection, msg interface{}) error {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return ErrAgentNoBindConnection
	}
	system := m.node.GetActorSystem()
	return system.PushMessage(pid, msg)
}

func (m *Gate) OnClose(entity network.IConnection, wrong error) error {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return ErrAgentNoBindConnection
	}

	system := m.node.GetActorSystem()
	return system.PushTask(pid, func(ctx iface.IContext) (wrong error) {
		_agent := ctx.Actor().(IAgent)
		wrong = _agent.OnClose(ctx)
		return
	})
}

func (m *Gate) Stop(ctx context.Context) error {
	if m.server != nil {
		_ = m.server.Stop()
	}
	return nil
}

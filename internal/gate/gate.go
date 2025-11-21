package gate

import (
	"fmt"
	"gas/internal/actor"
	"gas/internal/iface"
	"gas/pkg/network"
	"gas/pkg/utils/glog"
	"gas/pkg/utils/workers"
	"time"
)

func New(node iface.INode, factory Factory, opts ...Option) *Gate {
	gate := &Gate{
		factory:     factory,
		listener:    nil,
		opts:        loadOptions(opts...),
		actorSystem: node.GetSystem().(*actor.System),
	}
	return gate
}

type Gate struct {
	factory     Factory
	listener    network.IListener
	opts        *Options
	actorSystem *actor.System
}

func (m *Gate) Run() {
	m.listener = network.NewListener(m.opts.ProtoAddr, network.WithHandler(m))
	workers.Submit(func() {
		if err := m.listener.Serve(); err != nil {
			glog.Errorf("gate run listening on %s err:%v", m.listener.Addr(), err)
		}
	}, nil)

	glog.Infof("gate run listening on %s", m.listener.Addr())
}

func (m *Gate) OnOpen(entity network.IEntity) (err error) {
	if network.Count() > m.opts.MaxConn {
		return fmt.Errorf("maximum number of connections exceeded")
	}
	connection := NewConnection(m, entity)
	entity.SetContext(connection)
	if err = connection.OnConnect(); err != nil {
		return
	}
	return
}

func (m *Gate) OnTraffic(entity network.IEntity) (err error) {
	connection, _ := entity.Context().(*Connection)
	if connection == nil {
		return fmt.Errorf("no bind connection")
	}
	if err = connection.OnTraffic(); err != nil {
		return err
	}
	return
}
func (m *Gate) OnClose(entity network.IEntity, wrong error) (err error) {
	connection, _ := entity.Context().(*Connection)
	if connection == nil {
		return fmt.Errorf("no bind connection")
	}
	if err = connection.OnClose(wrong); err != nil {
		return
	}

	return
}

func (m *Gate) GracefulStop() {
	if m.listener == nil {
		return
	}

	_ = m.listener.Close()

	const checkInterval = 50 * time.Millisecond

	gracePeriod := m.opts.GracePeriod
	if gracePeriod <= 0 {
		gracePeriod = 5 * time.Second
	}

	timeout := time.After(gracePeriod)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		if network.Count() == 0 {
			return
		}

		select {
		case <-ticker.C:
		case <-timeout:
			return
		}
	}
}

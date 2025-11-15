package gate

import (
	"fmt"
	"gas/pkg/network"
	"gas/pkg/utils/workers"
	"time"
)

func New(producer AgentProducer, opts ...Option) *Gate {
	gate := &Gate{
		producer: producer,
		listener: nil,
		mgr:      NewSessionManager(),
		opts:     loadOptions(opts...),
	}
	return gate
}

type Gate struct {
	producer AgentProducer
	listener network.IListener
	mgr      ISessionManager
	opts     *Options
}

func (m *Gate) Run() {
	m.listener = network.NewListener(m.opts.ProtoAddr, network.WithHandler(m))
	workers.Submit(func() {
		if err := m.listener.Serve(); err != nil {
			fmt.Println(err)
		}
	}, nil)
}

func (m *Gate) Router() IRouter {
	return m.opts.Router
}

func (m *Gate) RegisterMessage(cmd uint8, act uint8, handler interface{}) error {
	return m.Router().Register(cmd, act, handler)
}

func (m *Gate) OnOpen(entity network.IEntity) (err error) {
	if m.mgr.Count() > m.opts.MaxConn {
		return fmt.Errorf("maximum number of connections exceeded")
	}
	session := NewSession(m, entity)
	m.mgr.Add(session)

	if err = session.OnConnect(); err != nil {
		return
	}
	return
}

func (m *Gate) OnTraffic(entity network.IEntity) (err error) {
	session, _ := m.mgr.Get(entity.ID())
	if session == nil {
		return fmt.Errorf("no bind session")
	}
	if err = session.OnTraffic(); err != nil {
		return err
	}
	return
}
func (m *Gate) OnClose(entity network.IEntity, wrong error) (err error) {
	session, _ := m.mgr.Get(entity.ID())
	if session == nil {
		return fmt.Errorf("no bind session")
	}
	if err = session.OnClose(wrong); err != nil {
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
		if m.mgr == nil || m.mgr.Count() == 0 {
			return
		}

		select {
		case <-ticker.C:
		case <-timeout:
			return
		}
	}
}

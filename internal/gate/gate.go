package gate

import (
	"fmt"
	"gas/pkg/actor"
	"gas/pkg/network"
	"gas/pkg/utils/workers"

	"google.golang.org/protobuf/proto"
)

func New(producer actor.Producer, opts ...Option) *Gate {
	gate := &Gate{
		producer: producer,
		listener: nil,
		mgr:      NewSessionManager(),
		opts:     loadOptions(opts...),
	}
	return gate
}

type Gate struct {
	listener network.IListener
	mgr      ISessionManager
	producer actor.Producer
	opts     *Options
}

func (m *Gate) Run() {

	m.listener = network.NewListener(m.opts.ProtoAddr, network.WithSessionFactory(func() network.ISession {
		return NewSession(m)
	}))

	workers.Submit(func() {
		if err := m.listener.Serve(); err != nil {
			fmt.Println(err)
		}
	}, nil)

}

func (m *Gate) Router() IRouter {
	return m.opts.Router
}

func (m *Gate) RegisterMessage(cmd uint8, act uint8, prototype proto.Message, handlerName string) error {
	return m.Router().Register(cmd, act, prototype, handlerName)
}

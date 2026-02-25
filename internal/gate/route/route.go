package route

import (
	"github.com/dzm2020/gas/internal/gate/protocol"
	"github.com/dzm2020/gas/internal/iface"
)

type IRouter interface {
	Route(session iface.ISession, message *protocol.Message) *iface.Pid
}

package gate

import (
	"errors"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/internal/session"
	"gas/pkg/glog"
	"gas/pkg/lib/xerror"
	"gas/pkg/network"

	"go.uber.org/zap"
)

var (
	ErrNotFoundEntity = errors.New("entity not found")
)

type Factory func() IAgent

type IAgent interface {
	iface.IActor
	OnConnectionOpen(ctx iface.IContext, s *session.Session) error
	OnConnectionMessage(ctx iface.IContext, s *session.Session, data []byte) error
	OnConnectionClose(ctx iface.IContext, s *session.Session) error
	PushMessage(ctx iface.IContext, s *session.Session, data []byte) error
	Shutdown(ctx iface.IContext, s *session.Session) error
}

var _ IAgent = (*Agent)(nil)

type Agent struct {
	iface.Actor
}

func (agent *Agent) OnConnectionOpen(ctx iface.IContext, s *session.Session) error {
	return nil
}
func (agent *Agent) OnConnectionMessage(ctx iface.IContext, s *session.Session, data []byte) error {
	return nil
}
func (agent *Agent) OnConnectionClose(ctx iface.IContext, s *session.Session) error {
	return nil
}
func (agent *Agent) PushMessage(ctx iface.IContext, s *session.Session, data []byte) error {
	entity := network.GetConnection(s.GetEntityId())
	if entity == nil {
		return xerror.Wrapf(ErrNotFoundEntity, "entity:%d", s.GetEntityId())
	}
	msg := protocol.New(uint8(s.Cmd), uint8(s.Act), data)
	msg.Index = s.GetIndex()
	msg.Error = uint16(s.GetCode())

	glog.Debug("发送消息到客户端", zap.Int64("entityId", s.GetEntityId()), zap.Any("msg", msg))

	return entity.Send(msg)

}
func (agent *Agent) Shutdown(ctx iface.IContext, s *session.Session) error {
	glog.Info("关闭网络连接", zap.Int64("entityId", s.GetEntityId()))

	entity := network.GetConnection(s.GetEntityId())
	if entity == nil {
		return xerror.Wrapf(ErrNotFoundEntity, "entity:%d", s.GetEntityId())
	}
	return entity.Close(nil)
}

package gate

import (
	"errors"

	"github.com/duke-git/lancet/v2/maputil"
	"github.com/dzm2020/gas/internal/gate/codec"
	"github.com/dzm2020/gas/internal/gate/protocol"
	"github.com/dzm2020/gas/internal/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"github.com/dzm2020/gas/pkg/network"

	"go.uber.org/zap"
)

//  note:agent 还是需要用actor来处理逻辑，这样可以集群下通过actor消息调用agent接口,更方便agent功能扩展

var (
	ErrNotFoundEntity = errors.New("entity not found")
)

type Factory func() IAgent

type IAgentHandler interface {
	OnData(ctx iface.IContext, s *session.Session, data []byte) error
}

var _ IAgentHandler = (*AgentHandler)(nil)

type AgentHandler struct {
}

func (a *AgentHandler) OnData(ctx iface.IContext, s *session.Session, data []byte) error {
	return nil
}

type IAgent interface {
	iface.IActor
	IAgentHandler
	SetMiddleware(middlewares []Middleware)
	Push(ctx iface.IContext, s *session.Session, data []byte) error
	Shutdown(ctx iface.IContext, s *session.Session) error
}

var _ IAgent = (*Agent)(nil)

type Agent struct {
	iface.Actor
	AgentHandler
	// middlewares 由 Gate 在创建 session 时注入，Encode 前按顺序执行
	middlewares []Middleware
}

func (agent *Agent) SetMiddleware(middlewares []Middleware) {
	agent.middlewares = middlewares
}

func (agent *Agent) Push(ctx iface.IContext, s *session.Session, data []byte) error {
	var err error
	entity := network.GetConnection(s.GetEntityId())
	if entity == nil {
		return xerror.Wrapf(ErrNotFoundEntity, "entity:%d", s.GetEntityId())
	}

	msg := protocol.New(uint8(s.Cmd), uint8(s.Act), data)
	msg.Index = s.GetIndex()
	msg.Error = uint16(s.GetCode())

	if len(agent.middlewares) > 0 {
		msg, err = RunBeforeEncode(agent.middlewares, msg)
		if err != nil {
			return xerror.Wrapf(err, "middleware BeforeEncode")
		}
		if msg == nil {
			return nil
		}
	}

	glog.Debug("发送消息到客户端", zap.Int64("entityId", s.GetEntityId()), zap.Any("msg", msg))

	var bytes []byte
	bytes, err = codec.Encode(msg)
	if err != nil {
		return xerror.Wrapf(err, "codec:%d", s.GetCode())
	}
	return entity.Send(bytes)
}

func (agent *Agent) SetValue(ctx iface.IContext, s *session.Session, data []byte) error {
	entity := network.GetConnection(s.GetEntityId())
	if entity == nil {
		return nil
	}
	ss := entity.Context().(*session.Session)
	ss.Values = s.GetValues()
	ss.Values = maputil.Merge(s.Values, s.GetValues())
	return nil
}

func (agent *Agent) Shutdown(ctx iface.IContext, s *session.Session) error {
	glog.Info("关闭网络连接", zap.Int64("entityId", s.GetEntityId()))
	entity := network.GetConnection(s.GetEntityId())
	if entity == nil {
		return xerror.Wrapf(ErrNotFoundEntity, "entity:%d", s.GetEntityId())
	}
	_ = entity.Close(nil)
	_ = ctx.Shutdown()
	return nil
}

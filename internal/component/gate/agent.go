package gate

import (
	"errors"

	"github.com/duke-git/lancet/v2/maputil"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/component/gate/session"
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
	OnData(ctx iface.IContext, s iface.ISession, data []byte) error
}

var _ IAgentHandler = (*AgentHandler)(nil)

type AgentHandler struct {
}

func (a *AgentHandler) OnData(ctx iface.IContext, s iface.ISession, data []byte) error {
	return nil
}

type IAgent interface {
	iface.IActor
	IAgentHandler
	AppendMiddleware(middlewares ...Middleware)
	Push(ctx iface.IContext, s iface.ISession, data []byte) error
	Shutdown(ctx iface.IContext, s iface.ISession) error
}

var _ IAgent = (*Agent)(nil)

type Agent struct {
	iface.Actor
	AgentHandler
	// middlewares 由 Gate 在创建 session 时注入，Encode 前按顺序执行
	middlewares []Middleware
}

func (agent *Agent) AppendMiddleware(middlewares ...Middleware) {
	agent.middlewares = append(agent.middlewares, middlewares...)
}

func (agent *Agent) Push(ctx iface.IContext, s iface.ISession, data []byte) error {
	ses := s.(*session.Session)
	var err error
	entity := network.GetConnection(ses.GetId())
	if entity == nil {
		return xerror.Wrapf(ErrNotFoundEntity, "entity:%d", ses.GetId())
	}

	msg := protocol.New(ses.GetCmd(), ses.GetAct(), data)
	msg.Index = ses.GetIndex()
	msg.Error = uint16(ses.GetErrCode())

	if len(agent.middlewares) > 0 {
		msg, err = RunBeforeEncode(agent.middlewares, msg)
		if err != nil {
			return xerror.Wrapf(err, "middleware BeforeEncode")
		}
		if msg == nil {
			return nil
		}
	}

	glog.Debug("发送消息到客户端", zap.Int64("entityId", ses.GetId()), zap.Any("msg", msg))

	var bytes []byte
	bytes, err = codec.Encode(msg)
	if err != nil {
		return xerror.Wrapf(err, "codec")
	}
	return entity.Send(bytes)
}

func (agent *Agent) SetValue(ctx iface.IContext, s iface.ISession, data []byte) error {
	ses := s.(*session.Session)
	entity := network.GetConnection(ses.GetId())
	if entity == nil {
		return nil
	}
	ss, ok := entity.Context().(*session.Session)
	if ss == nil || !ok {
		return nil
	}
	ss.Values = ses.GetValues()
	ss.Values = maputil.Merge(ss.Values, ses.GetValues())
	return nil
}

func (agent *Agent) Shutdown(ctx iface.IContext, s iface.ISession) error {
	ses := s.(*session.Session)
	glog.Info("关闭网络连接", zap.Int64("entityId", ses.GetId()))
	entity := network.GetConnection(ses.GetId())
	if entity == nil {
		return xerror.Wrapf(ErrNotFoundEntity, "entity:%d", ses.GetId())
	}
	_ = entity.Close(nil)
	_ = ctx.Shutdown()
	return nil
}

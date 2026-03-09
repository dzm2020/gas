package gate

import (
	"encoding/json"
	"errors"

	"github.com/duke-git/lancet/v2/maputil"
	"github.com/dzm2020/gas/internal/component/gate/codec"
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
	Init(s *session.Session, entity network.IConnection, middlewares ...Middleware)
	GetEntity() network.IConnection
	GetSession() *session.Session
	AppendMiddleware(middlewares ...Middleware)
	Push(ctx iface.IContext, _ []byte) error
	Shutdown(ctx iface.IContext, _ []byte) error
}

var _ IAgent = (*Agent)(nil)

type Agent struct {
	iface.Actor
	AgentHandler
	session     *session.Session
	entity      network.IConnection
	middlewares []Middleware // middlewares 由 Gate 在创建 session 时注入，Encode 前按顺序执行
}

func (agent *Agent) GetEntity() network.IConnection {
	return agent.entity
}

func (agent *Agent) GetSession() *session.Session {
	return agent.session
}

func (agent *Agent) Init(session *session.Session, entity network.IConnection, middlewares ...Middleware) {
	agent.session = session
	agent.entity = entity
	agent.middlewares = middlewares
}

func (agent *Agent) AppendMiddleware(middlewares ...Middleware) {
	agent.middlewares = append(agent.middlewares, middlewares...)
}

func (agent *Agent) Push(ctx iface.IContext, data []byte) (err error) {
	msg, _, _ := codec.Decode(data)
	if len(agent.middlewares) > 0 {
		msg, err = RunBeforeEncode(agent.middlewares, msg)
		if err != nil {
			return xerror.Wrapf(err, "middleware BeforeEncode")
		}
		if msg == nil {
			return nil
		}
	}

	glog.Debug("发送消息到客户端", zap.Int64("entityId", agent.GetEntity().ID()), zap.Any("msg", msg))

	var bin []byte
	bin, err = codec.Encode(msg)
	if err != nil {
		return xerror.Wrapf(err, "codec")
	}
	return agent.GetEntity().Send(bin)
}

func (agent *Agent) SetValue(ctx iface.IContext, data []byte) error {
	values := make(map[string]string)
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	agent.session.Values = maputil.Merge(agent.session.Values, values)
	return nil
}

func (agent *Agent) Shutdown(ctx iface.IContext, _ []byte) error {
	glog.Info("关闭网络连接", zap.Int64("entityId", agent.GetEntity().ID()))
	_ = agent.GetEntity().Close(nil)
	_ = ctx.Shutdown()
	return nil
}

package agent

import (
	"encoding/json"

	"github.com/duke-git/lancet/v2/maputil"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/middleware"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"github.com/dzm2020/gas/pkg/network"

	"go.uber.org/zap"
)

// note:agent 还是需要用actor来处理逻辑，这样可以集群下通过actor消息调用agent接口,更方便agent功能扩展

type IAgent interface {
	GetEntity() network.IConnection
	GetSession() *session.Session
	SetMiddleware(chain []middleware.IMiddleware)
	AppendMiddleware(middlewares ...middleware.IMiddleware)
	GetMiddleware() []middleware.IMiddleware
}

var _ IAgent = (*Agent)(nil)

func New(entity network.IConnection, handler IHandler) *Agent {
	return &Agent{
		entity:   entity,
		IHandler: handler,
	}
}

type Agent struct {
	iface.Actor
	IHandler
	session     *session.Session
	entity      network.IConnection
	middlewares []middleware.IMiddleware // middlewares 由 Gate 在创建 session 时注入，Encode 前按顺序执行
}

func (agent *Agent) OnInit(ctx iface.IContext, params []interface{}) error {
	entity := agent.GetEntity()
	s := ctx.System().SessionFactory().FromRaw(ctx, &iface.Session{
		Id: entity.ID(),
	})
	agent.session = s.(*session.Session)
	agent.session.SetAgent(ctx.ID())
	agent.entity = entity
	return agent.IHandler.OnInit(ctx, agent)
}

func (agent *Agent) OnData(ctx iface.IContext, msg *protocol.Message) (err error) {
	//  执行中间件
	msg, err = middleware.RunAfterDecode(agent.GetMiddleware(), msg)
	if err != nil || msg == nil {
		return err
	}
	//  存储客户端消息
	agent.session.SetCmd(msg.Cmd)
	agent.session.SetAct(msg.Act)
	agent.session.SetIndex(msg.Index)
	//  回调
	return agent.IHandler.OnRoute(ctx, agent, msg.Data)
}

func (agent *Agent) OnStop(ctx iface.IContext) error {
	return agent.IHandler.OnStop(ctx, agent)
}

func (agent *Agent) GetEntity() network.IConnection {
	return agent.entity
}

func (agent *Agent) GetSession() *session.Session {
	return agent.session
}

func (agent *Agent) AppendMiddleware(middlewares ...middleware.IMiddleware) {
	agent.middlewares = append(agent.middlewares, middlewares...)
}

// SetMiddleware 替换当前 agent 的整条 middleware 链（测试或自定义 agent 用）。
func (agent *Agent) SetMiddleware(chain []middleware.IMiddleware) {
	agent.middlewares = chain
}

func (agent *Agent) GetMiddleware() []middleware.IMiddleware {
	return agent.middlewares
}

func (agent *Agent) Push(ctx iface.IContext, data []byte) (err error) {
	msg, _, _ := codec.Decode(data)
	if len(agent.middlewares) > 0 {
		msg, err = middleware.RunBeforeEncode(agent.middlewares, msg)
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

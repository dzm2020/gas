// 本文件实现连接对应的 Agent Actor：会话创建、消息解码后走中间件与 IHandler、以及 Push/SetValue/Shutdown 的 Actor 调用。
package agent

import (
	"encoding/json"
	"errors"

	"github.com/duke-git/lancet/v2/maputil"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/middleware"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/pb"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"github.com/dzm2020/gas/pkg/network"

	"go.uber.org/zap"
)

// IAgent 对外暴露的 Agent 能力：获取连接、Session、以及中间件链的配置（便于集群下通过 Actor 消息扩展）。
type IAgent interface {
	GetEntity() network.IConnection
	GetSession() *session.Session
	SetMiddleware(chain []middleware.IMiddleware)
	AppendMiddleware(middlewares ...middleware.IMiddleware)
	GetMiddleware() []middleware.IMiddleware
}

var _ IAgent = (*Agent)(nil)

// New 构造 Agent，绑定连接与业务 IHandler；由 Gate.OnConnect 调用并 Spawn 为 Actor。
func New(entity network.IConnection, handler IHandler) *Agent {
	return &Agent{
		entity:   entity,
		IHandler: handler,
	}
}

// Agent 每个客户端连接对应的 Actor：持有 entity、Session、中间件链，消息经 AfterDecode 后交给 IHandler.OnRoute，Push 前经 BeforeEncode。
type Agent struct {
	iface.Actor
	IHandler
	session     *session.Session
	entity      network.IConnection
	middlewares []middleware.IMiddleware
}

// OnInit 初始化 Session（含当前 Agent Pid），并调用 IHandler.OnInit。
func (agent *Agent) OnInit(ctx iface.IContext, params []interface{}) error {
	entity := agent.GetEntity()

	agent.session = session.New(&pb.Session{
		Id:     entity.ID(),
		Agent:  ctx.ID(),
		Values: make(map[string]string),
	}, ctx)

	agent.entity = entity
	return agent.IHandler.OnInit(ctx, agent)
}

// OnData 收到 Gate 投递的协议消息：先走 AfterDecode 中间件，再 SetMessage 并调用 IHandler.OnRoute(ctx, agent, msg.Data)。
func (agent *Agent) OnData(ctx iface.IContext, msg *protocol.Message) (err error) {
	msg, err = middleware.RunAfterDecode(agent.GetMiddleware(), msg)
	if err != nil || msg == nil {
		return err
	}
	agent.session.SetMessage(msg)
	return agent.IHandler.OnRoute(ctx, agent, msg.Data)
}

// OnStop 委托 IHandler.OnStop 做清理。
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

// Push 由系统/对端调用：将 data 解包为 Message，经 BeforeEncode 中间件后编码并发送到 entity；用于响应或集群转发到客户端。
func (agent *Agent) Push(ctx iface.IContext, data []byte) (err error) {
	msg, _, decodeErr := codec.Decode(data)
	if decodeErr != nil {
		glog.Warn("agent.Push decode failed", zap.Int64("entityId", agent.GetEntity().ID()), zap.Error(decodeErr))
		return xerror.Wrapf(decodeErr, "codec.Decode")
	}
	if msg == nil {
		glog.Warn("agent.Push decode returned nil message", zap.Int64("entityId", agent.GetEntity().ID()))
		return errors.New("codec.Decode: incomplete or empty message")
	}
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

// SetValue 处理对端同步的 Values：data 为 JSON map，与当前 session.Values 合并。
func (agent *Agent) SetValue(ctx iface.IContext, data []byte) error {
	values := make(map[string]string)
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	agent.session.Values = maputil.Merge(agent.session.Values, values)
	return nil
}

// Shutdown 关闭网络连接并关闭当前 Actor 进程。
func (agent *Agent) Shutdown(ctx iface.IContext, _ []byte) error {
	glog.Info("关闭网络连接", zap.Int64("entityId", agent.GetEntity().ID()))
	_ = agent.GetEntity().Close(nil)
	_ = ctx.Shutdown()
	return nil
}

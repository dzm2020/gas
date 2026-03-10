// 本文件实现连接对应的 Agent Actor：会话创建、消息解码后走中间件与 IHandler、以及 Push/SetValue/Shutdown 的 Actor 调用。
package agent

import (
	"encoding/json"
	"errors"

	"github.com/duke-git/lancet/v2/maputil"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	gateiface "github.com/dzm2020/gas/internal/component/gate/gateiface"
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

type IRemoteHandler interface {
	HandlerPush(ctx iface.IContext, data []byte) error
	HandlerSetValue(ctx iface.IContext, data []byte) error
	HandlerShutdown(ctx iface.IContext, data []byte) error
}

var _ gateiface.IAgent = (*Agent)(nil)

// New 构造 Agent，绑定连接与业务 IHandler；由 Gate.OnConnect 调用并 Spawn 为 Actor。
func New(entity network.IConnection, handler IHandler) *Agent {
	return &Agent{
		entity:   entity,
		IHandler: handler,
	}
}

var _ IRemoteHandler = (*Agent)(nil)

// Agent 每个客户端连接对应的 Actor：持有 entity、Session、中间件链，消息经 AfterDecode 后交给 IHandler.OnRoute，Push 前经 BeforeEncode。
type Agent struct {
	iface.Actor
	IHandler
	ctx         iface.IContext
	session     *session.Session
	entity      network.IConnection
	middlewares []gateiface.IMiddleware
}

// OnInit 初始化 Session（含当前 Agent Pid），并调用 IHandler.OnInit。
func (agent *Agent) OnInit(ctx iface.IContext, params []interface{}) error {
	agent.ctx = ctx
	entity := agent.GetEntity()
	agent.session = session.New(&pb.Session{
		Id:     entity.ID(),
		Agent:  ctx.ID(),
		Values: make(map[string]string),
	}, ctx)

	agent.entity = entity
	return agent.IHandler.OnInit(agent)
}

// OnData 收到 Gate 投递的协议消息：先走 AfterDecode 中间件，再 SetMessage 并调用 IHandler.OnRoute(ctx, agent, msg.Data)。
func (agent *Agent) OnData(ctx iface.IContext, msg *protocol.Message) (err error) {
	msg, err = middleware.RunAfterDecode(agent.GetMiddleware(), agent, msg)
	if err != nil || msg == nil {
		return err
	}
	agent.session.SetMessage(msg)
	return agent.IHandler.OnRoute(agent, msg.Data)
}
func (agent *Agent) Context() iface.IContext {
	return agent.ctx
}

// OnStop 委托 IHandler.OnStop 做清理。
func (agent *Agent) OnStop(ctx iface.IContext) error {
	return agent.IHandler.OnStop(agent)
}

func (agent *Agent) GetEntity() network.IConnection {
	return agent.entity
}

func (agent *Agent) GetSession() *session.Session {
	return agent.session
}

func (agent *Agent) AppendMiddleware(middlewares ...gateiface.IMiddleware) {
	agent.middlewares = append(agent.middlewares, middlewares...)
}

// SetMiddleware 替换当前 agent 的整条 middleware 链（测试或自定义 agent 用）。
func (agent *Agent) SetMiddleware(chain []gateiface.IMiddleware) {
	agent.middlewares = chain
}

func (agent *Agent) GetMiddleware() []gateiface.IMiddleware {
	return agent.middlewares
}

// Push 由系统/对端调用：将 data 解包为 Message，经 BeforeEncode 中间件后编码并发送到 entity；用于响应或集群转发到客户端。
func (agent *Agent) Push(msg *protocol.Message) (err error) {
	if len(agent.middlewares) > 0 {
		msg, err = middleware.RunBeforeEncode(agent.middlewares, agent, msg)
		if err != nil {
			return xerror.Wrapf(err, "middleware BeforeEncode")
		}
		if msg == nil {
			return nil
		}
	}
	var bin []byte
	bin, err = codec.Encode(msg)
	if err != nil {
		return xerror.Wrapf(err, "codec")
	}
	return agent.GetEntity().Send(bin)
}

// SetValues 处理对端同步的 Values：data 为 JSON map，与当前 session.Values 合并。
func (agent *Agent) SetValues(values map[string]string) error {
	agent.session.Values = maputil.Merge(agent.session.Values, values)
	return nil
}

// Shutdown 关闭网络连接并关闭当前 Actor 进程。
func (agent *Agent) Shutdown() error {
	_ = agent.GetEntity().Close(nil)
	_ = agent.ctx.Shutdown()

	glog.Info("关闭网络连接", zap.Int64("entityId", agent.GetEntity().ID()))
	return nil
}

//////////////////////////////////////////////////////////////////远程消息回调/////////////////////////////////////////////////////////////////////////////

func (agent *Agent) HandlerPush(ctx iface.IContext, data []byte) error {
	msg, _, decodeErr := codec.Decode(data)
	if decodeErr != nil {
		return xerror.Wrapf(decodeErr, "codec.Decode")
	}
	if msg == nil {
		return errors.New("codec.Decode: incomplete or empty message")
	}
	return agent.Push(msg)
}

func (agent *Agent) HandlerSetValue(ctx iface.IContext, data []byte) error {
	values := make(map[string]string)
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	return agent.SetValues(values)
}

func (agent *Agent) HandlerShutdown(ctx iface.IContext, data []byte) error {
	return agent.Shutdown()
}

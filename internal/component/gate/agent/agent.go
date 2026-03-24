// Package agent
// @Description: 本文件实现连接对应的 Agent Actor：会话创建、消息解码后走中间件与 IHandler、以及 Push/SetValue/Shutdown 的 Actor 调用。
package agent

import (
	"encoding/json"
	"errors"

	"github.com/duke-git/lancet/v2/maputil"
	"github.com/dzm2020/gas/api/pb"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/component/gate/middleware"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/uid"
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

// New
//
//	@Description: 构造 Agent，由 Gate.OnConnect 调用并 Spawn。
//	@param entity
//	@param handler
//	@return *Agent
func New(entity network.IConnection, handler gateiface.IBusinessHandler) *Agent {
	if handler == nil {
		handler = new(gateiface.AgentHandler)
	}
	return &Agent{
		entity:           entity,
		IBusinessHandler: handler,
	}
}

var _ IRemoteHandler = (*Agent)(nil)

// Agent 每个客户端连接对应的 Actor：持有 entity、Session、中间件链，消息经 AfterDecode 后交给 IHandler.OnRoute，Push 前经 BeforeEncode。
type Agent struct {
	iface.Actor
	gateiface.IBusinessHandler
	ctx         iface.IContext
	session     *pb.Session
	entity      network.IConnection
	middlewares []gateiface.IMiddleware
}

// OnInit
//
//	@Description: 初始化 Session 并调用 IHandler.OnInit。
//	@receiver agent
//	@param ctx
//	@param params
//	@return error
func (agent *Agent) OnInit(ctx iface.IContext, params []interface{}) error {
	agent.ctx = ctx
	entity := agent.GetEntity()
	//  session是跨集群的所以要保证集群唯一性
	sessionID, err := uid.NextId()
	if err != nil {
		return xerror.Wrapf(err, "uid.NextId for session")
	}
	agent.session = &pb.Session{
		Id:     sessionID,
		Agent:  ctx.ID(),
		Values: make(map[string]string),
	}

	agent.entity = entity
	return agent.IBusinessHandler.OnInit(agent)
}

// OnData
//
//	@Description: 经 AfterDecode 后 SetMessage 并调用 IHandler.OnRoute。
//	@receiver agent
//	@param ctx
//	@param msg
//	@return err
func (agent *Agent) OnData(msg *protocol.Message) (err error) {
	msg, err = middleware.RunAfterDecode(agent.GetMiddleware(), agent, msg)
	if err != nil || msg == nil {
		return err
	}

	agent.prepareRequest(msg)

	return agent.IBusinessHandler.OnRoute(agent, msg.Data)
}

func (agent *Agent) prepareRequest(msg *protocol.Message) {
	session.SetMessage(agent.session, msg)

	actorMessage := agent.ctx.Message()
	actorMessage.To = agent.ctx.ID()
	actorMessage.Data = msg.Data
	actorMessage.Session = agent.GetSession()
}

// OnStop
//
//	@Description: 委托 IHandler.OnStop 清理。
//	@receiver agent
//	@param ctx
//	@return error
func (agent *Agent) OnStop(ctx iface.IContext) error {
	return agent.IBusinessHandler.OnStop(agent)
}

// GetEntity
//
//	@Description:
//	@receiver agent
//	@return network.IConnection
func (agent *Agent) GetEntity() network.IConnection {
	return agent.entity
}

// Context
//
//	@Description:获取agent actor context
//	@receiver agent
//	@return iface.IContext
func (agent *Agent) Context() iface.IContext {
	return agent.ctx
}

// GetSession
//
//	@Description:获取session
//	@receiver agent
//	@return *session.Session
func (agent *Agent) GetSession() *pb.Session {
	return agent.session
}

// AppendMiddleware
//
//	@Description: 添加中间件
//	@receiver agent
//	@param middlewares
func (agent *Agent) AppendMiddleware(middlewares ...gateiface.IMiddleware) {
	agent.middlewares = append(agent.middlewares, middlewares...)
}

// SetMiddleware
//
//	@Description: 替换整条 middleware 链。
//	@receiver agent
//	@param chain
func (agent *Agent) SetMiddleware(chain []gateiface.IMiddleware) {
	agent.middlewares = chain
}

// GetMiddleware
//
//	@Description: 获取中间件
//	@receiver agent
//	@return []gateiface.IMiddleware
func (agent *Agent) GetMiddleware() []gateiface.IMiddleware {
	return agent.middlewares
}

// Push
//
//	@Description: 经 BeforeEncode 后编码并发送到连接。
//	@receiver agent
//	@param msg
//	@return err
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

// SetValues
//
//	@Description: 合并对端同步的 Values 到 session。
//	@receiver agent
//	@param values
//	@return error
func (agent *Agent) SetValues(values map[string]string) error {
	agent.session.Values = maputil.Merge(agent.session.Values, values)
	return nil
}

// Shutdown
//
//	@Description: 关闭连接并关闭当前 Actor。
//	@receiver agent
//	@return error
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

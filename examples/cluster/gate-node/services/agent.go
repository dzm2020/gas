package services

import (
	"github.com/dzm2020/gas/examples/cluster/common"
	"github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/glog"
	"go.uber.org/zap"
)

var _ gateiface.IBusinessHandler = (*BusinessHandler)(nil)

type BusinessHandler struct {
}

func (a *BusinessHandler) OnInit(agent gateiface.IAgent) error {
	agent.Context().Actor()
	glog.Info("AgentHandler onInit")
	return nil
}

func (a *BusinessHandler) OnStop(agent gateiface.IAgent) error {
	glog.Info("AgentHandler onStop")
	return nil
}

// OnRoute 网关业务入口：s 由框架注入且非 nil；下行客户端用 session.Response/ResponseErr/Push；转发游戏节点用 ctx.ForwardMessage，会话快照在消息体 Session 字段中。
func (a *BusinessHandler) OnRoute(agent gateiface.IAgent, s iface.ISession, data []byte) error {
	glog.Info("OnData", zap.Int64("sessionId", s.GetId()))

	////  test cluster actor  message
	//ctx := agent.Context()
	//nodeId, err := common.Node.Cluster().Select("UserMgr", cluster.RouteRandom)
	//if err != nil {
	//	return nil
	//}
	//pid := iface.NewPidWithName("UserMgr", nodeId)
	//if err := ctx.Send(pid, "OnHandlerAsyncMessage", &common.ChatMessageRequest{Content: "async message"}); err != nil {
	//	glog.Error("OnData", zap.Error(err))
	//	return err
	//}
	//
	//ctx.SetCallTimeout(time.Second * 60)
	//response := &common.ChatMessageResponse{}
	//if err := ctx.Call(pid, "OnHandlerSyncMessage", &common.ChatMessageRequest{Content: "sync message"}, response); err != nil {
	//	glog.Error("OnData", zap.Error(err))
	//	return err
	//}
	//glog.Info("OnHandlerSyncMessage", zap.Any("response", response))

	// 下行示例：优先 session 包封装，并检查 error
	if err := session.ResponseErr(s, 111); err != nil {
		return err
	}
	if err := session.Response(s, []byte("test response")); err != nil {
		return err
	}
	if err := session.Push(s, 1, 1, []byte("test push")); err != nil {
		return err
	}

	// test cluster actor session
	//  负载均衡
	ctx := agent.Context()
	nodeId, err := common.Node.Cluster().Select("UserMgr", cluster.RouteRandom)
	if err != nil {
		return nil
	}
	pid := iface.NewPidWithName("UserMgr", nodeId)
	//  转发
	return ctx.ForwardMessage(pid, "OnHandlerLogin")
}

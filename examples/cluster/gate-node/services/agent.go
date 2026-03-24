package services

import (
	"time"

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

func (a *BusinessHandler) OnRoute(agent gateiface.IAgent, data []byte) error {
	s := agent.GetSession()

	glog.Info("OnData", zap.Int64("sessionId", s.GetId()))

	//  test cluster actor  message
	ctx := agent.Context()
	nodeId, err := common.Node.Cluster().Select("UserMgr", cluster.RouteRandom)
	if err != nil {
		return nil
	}
	pid := iface.NewPidWithName("UserMgr", nodeId)
	if err := ctx.Send(pid, "OnHandlerAsyncMessage", &common.ChatMessageRequest{Content: "async message"}); err != nil {
		glog.Error("OnData", zap.Error(err))
		return err
	}

	ctx.SetCallTimeout(time.Second * 60)
	response := &common.ChatMessageResponse{}
	if err := ctx.Call(pid, "OnHandlerSyncMessage", &common.ChatMessageRequest{Content: "sync message"}, response); err != nil {
		glog.Error("OnData", zap.Error(err))
		return err
	}
	glog.Info("OnHandlerSyncMessage", zap.Any("response", response))

	//  test gate send message  to client
	session.ResponseErr(agent.Context(), agent.GetSession(), 111)
	session.Response(agent.Context(), agent.GetSession(), []byte("test response"))
	session.Push(agent.Context(), agent.GetSession(), 1, 1, []byte("test push"))

	// test cluster actor session
	////  负载均衡
	//ctx := agent.Context()
	//nodeId, err := common.Node.Cluster().Select("UserMgr", cluster.RouteRandom)
	//if err != nil {
	//	return nil
	//}
	//pid := iface.NewPidWithName("UserMgr", nodeId)
	////  转发
	//return ctx.ForwardMessage(pid, "OnHandlerLogin")
	return nil
}

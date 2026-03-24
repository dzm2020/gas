package services

import (
	"github.com/dzm2020/gas/examples/cluster/common"
	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"go.uber.org/zap"
)

//var (
//	Router = actor.NewRouter()
//)

func init() {
	//Router.Register(int64(protocol.CmdAct(1, 1)), OnHandlerLogin)
	//Router.Register(2, OnHandlerTestSend)
	//Router.Register(3, OnHandlerTestRequest)
}

func NewGameActor() iface.IActor {
	return &GameActor{}
}

type GameActor struct {
	iface.Actor
}

func (a *GameActor) OnInit(ctx iface.IContext, params []interface{}) error {
	_ = ctx.Named("UserMgr")

	//ctx.SetRouter(Router)

	return nil
}

// OnMessage 澶勭悊娑堟伅
func (a *GameActor) OnMessage(ctx iface.IContext, msg interface{}) error {
	glog.Infof("user mgr OnMessage received:  data=%+v\n", msg)
	return nil
}

// OnHandlerLogin 集群侧会话型 handler：s 非 nil，回写客户端请用 gate/session.Response（或 ResponseErr/Push），勿手写 Send。
func (a *GameActor) OnHandlerLogin(ctx iface.IContext, s iface.ISession, request *common.LoginRequest) error {

	glog.Infof("user mgr OnHandlerLogin received")

	request.Uid = 123456

	if err := session.Response(s, []byte("response message")); err != nil {
		return err
	}

	return nil
}

func (a *GameActor) OnHandlerSyncMessage(ctx iface.IContext, request *common.ChatMessageRequest, response *common.ChatMessageResponse) error {
	glog.Info("OnHandlerSyncMessage", zap.Any("req", request))
	response.Content = "response"
	return nil
}

func (a *GameActor) OnHandlerAsyncMessage(ctx iface.IContext, request *common.ChatMessageRequest) error {
	glog.Info("OnHandlerAsyncMessage", zap.Any("req", request))
	return nil
}

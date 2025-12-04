// Package actor
// @Description:

package actor

import (
	"errors"
	"fmt"
	"gas/internal/iface"
	"gas/pkg/lib"
	"time"
)

type IMessageInvoker interface {
	InvokerMessage(message interface{}) error
}

// contextConfig context 配置
type contextConfig struct {
	pid         *iface.Pid
	actor       iface.IActor
	middlewares []iface.TaskMiddleware
	router      iface.IRouter
	system      *System
	process     iface.IProcess
}

func newBaseActorContext(cfg *contextConfig) *baseActorContext {
	ctx := &baseActorContext{
		pid:          cfg.pid,
		actor:        cfg.actor,
		middlerWares: cfg.middlewares,
		router:       cfg.router,
		system:       cfg.system,
		process:      cfg.process,
		timerMgr:     newTimerManager(),
		node:         cfg.system.GetNode(),
	}
	return ctx
}

type baseActorContext struct {
	pid          *iface.Pid
	name         string
	actor        iface.IActor
	middlerWares []iface.TaskMiddleware
	msg          interface{}
	router       iface.IRouter
	system       *System
	process      iface.IProcess // 保存自己的 process 引用
	timerMgr     *timerManager  // 定时器管理器
	node         iface.INode
}

func (a *baseActorContext) ID() *iface.Pid {
	return a.pid
}
func (a *baseActorContext) Node() iface.INode {
	return a.node
}
func (a *baseActorContext) InvokerMessage(msg interface{}) error {
	a.msg = msg

	switch m := msg.(type) {
	case *iface.TaskMessage:
		return chain(a.middlerWares, m.Task)(a)
	case *iface.Message:
		if a.router != nil && a.router.HasRoute(uint16(m.GetId())) {
			_, err := a.execHandler(m)
			return err
		}
	case *iface.SyncMessage:
		if a.router != nil && a.router.HasRoute(uint16(m.GetId())) {
			data, err := a.execHandler(m.Message)
			m.Response(data, err)
			return err
		}
	}

	return a.actor.OnMessage(a, msg)
}

func (a *baseActorContext) execHandler(msg *iface.Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("msg is nil")
	}
	return a.router.Handle(a, uint16(msg.GetId()), msg.GetData())
}

func (a *baseActorContext) Actor() iface.IActor {
	return a.actor
}

func (a *baseActorContext) GetSerializer() lib.ISerializer {
	if a.system != nil {
		return a.system.GetSerializer()
	}
	return lib.Json // 默认序列化器
}

func (a *baseActorContext) Exit() {
	// 取消所有定时器
	if a.timerMgr != nil {
		a.timerMgr.CancelAllTimers()
	}
	if a.system != nil {
		a.system.unregisterProcess(a.pid)
	}
	_ = a.actor.OnStop(a)
}

func (a *baseActorContext) Send(to *iface.Pid, msgId uint16, request interface{}) error {
	if to == nil {
		return fmt.Errorf("target pid is nil")
	}
	if request == nil {
		return fmt.Errorf("request is nil")
	}
	if a.system == nil {
		return fmt.Errorf("system is not initialized")
	}

	message := a.buildMessage(to, msgId, request)
	if message == nil {
		return fmt.Errorf("build message failed")
	}

	if n := a.system.GetNode(); n != nil {
		return n.GetRemote().Send(message)
	}
	return a.system.Send(message)
}

func (a *baseActorContext) Select(service string, strategy iface.RouteStrategy) (*iface.Pid, error) {
	node := a.system.GetNode()
	if node == nil {
		return nil, errors.New("node is nil")
	}
	remote := node.GetRemote()
	if remote == nil {
		return nil, errors.New("remote is nil")
	}
	return remote.Select(service, strategy)
}

func (a *baseActorContext) SendService(service string, msgId uint16, request interface{}, strategy iface.RouteStrategy) error {
	if len(service) <= 0 {
		return fmt.Errorf("target pid is nil")
	}
	if request == nil {
		return fmt.Errorf("request is nil")
	}
	if a.system == nil {
		return fmt.Errorf("system is not initialized")
	}
	to, err := a.Select(service, strategy)
	if err != nil {
		return err
	}
	message := a.buildMessage(to, msgId, request)
	if message == nil {
		return fmt.Errorf("build message failed")
	}

	if n := a.system.GetNode(); n != nil {
		return n.GetRemote().Send(message)
	}
	return a.system.Send(message)
}

func (a *baseActorContext) Request(to *iface.Pid, msgId uint16, request interface{}, reply interface{}) error {
	if to == nil || request == nil || reply == nil {
		return fmt.Errorf("invalid parameters")
	}
	if a.system == nil {
		return fmt.Errorf("system is not initialized")
	}

	message := a.buildMessage(to, msgId, request)
	if message == nil {
		return fmt.Errorf("build message failed")
	}

	timeout := 5 * time.Second
	var response *iface.RespondMessage
	if n := a.system.GetNode(); n != nil {
		response = n.GetRemote().Request(message, timeout)
	} else {
		response = a.system.Request(message, timeout)
	}

	if errMsg := response.GetError(); errMsg != "" {
		return fmt.Errorf("request failed: %s", errMsg)
	}

	if data := response.GetData(); len(data) > 0 {
		if err := a.system.GetSerializer().Unmarshal(data, reply); err != nil {
			return fmt.Errorf("unmarshal reply failed: %w", err)
		}
	}
	return nil
}

func (a *baseActorContext) buildMessage(to *iface.Pid, msgId uint16, request interface{}) *iface.Message {
	if m, ok := request.(*iface.Message); ok {
		return m
	}

	ser := a.system.GetSerializer()
	requestData, err := ser.Marshal(request)
	if err != nil {
		return nil
	}

	return &iface.Message{
		To:   to,
		From: a.pid,
		Id:   uint32(msgId),
		Data: requestData,
	}
}

// RegisterTimer 注册定时器，时间到后通过 pushTask 通知 baseActorContext 然后执行回调
// 这是 IContext 接口要求的方法，内部调用 AfterFunc
func (a *baseActorContext) RegisterTimer(duration time.Duration, callback iface.Task) (int64, error) {
	return a.timerMgr.AfterFunc(a.process, duration, callback)
}

// AfterFunc 注册一次性定时器
func (a *baseActorContext) AfterFunc(duration time.Duration, callback iface.Task) (int64, error) {
	return a.timerMgr.AfterFunc(a.process, duration, callback)
}

// TickFunc 注册周期性定时器，每隔指定时间间隔执行一次回调
func (a *baseActorContext) TickFunc(interval time.Duration, callback iface.Task) (int64, error) {
	return a.timerMgr.TickFunc(a.process, interval, callback)
}

// CancelTimer 取消定时器
func (a *baseActorContext) CancelTimer(timerID int64) bool {
	return a.timerMgr.CancelTimer(timerID)
}

// CancelAllTimers 取消所有定时器
func (a *baseActorContext) CancelAllTimers() {
	a.timerMgr.CancelAllTimers()
}

func (a *baseActorContext) RegisterName(name string, isGlobal bool) error {
	if len(name) == 0 {
		return fmt.Errorf("name cannot be empty")
	}
	if a.system == nil {
		return fmt.Errorf("system is not initialized")
	}

	// 本地注册：更新 pid 的名称并在 system 中注册
	oldName := a.pid.Name
	a.pid.Name = name

	// 如果之前有名称，先从 nameDict 中删除旧名称
	if oldName != "" && oldName != name {
		a.system.nameDict.Delete(oldName)
	}

	// 在本地 system 中注册新名称
	a.system.nameDict.Set(name, a.process)

	// 如果是全局注册，还需要在远程注册
	if isGlobal {
		if a.node == nil {
			return fmt.Errorf("node is not initialized")
		}
		remote := a.node.GetRemote()
		if remote == nil {
			return fmt.Errorf("remote is not initialized")
		}
		if err := remote.RegistryName(name); err != nil {
			// 如果远程注册失败，回滚本地注册
			a.system.nameDict.Delete(name)
			a.pid.Name = oldName
			if oldName != "" {
				a.system.nameDict.Set(oldName, a.process)
			}
			return fmt.Errorf("remote registry name failed: %w", err)
		}
	}

	return nil
}

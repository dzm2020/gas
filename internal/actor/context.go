// Package actor
// @Description:

package actor

import (
	"fmt"
	"gas/internal/iface"
	"gas/pkg/utils/serializer"
	"time"
)

type IMessageInvoker interface {
	InvokerMessage(message interface{}) error
}

func newBaseActorContext(pid *iface.Pid, actor iface.IActor, middlerWares []iface.TaskMiddleware, r iface.IRouter, system *System) *baseActorContext {
	ctx := &baseActorContext{
		pid:          pid,
		actor:        actor,
		middlerWares: middlerWares,
		router:       r,
		system:       system,
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
}

func (a *baseActorContext) ID() *iface.Pid {
	return a.pid
}

func (a *baseActorContext) InvokerMessage(msg interface{}) error {
	a.msg = msg

	switch m := msg.(type) {
	case *iface.TaskMessage:
		_task := chain(a.middlerWares, m.Task)
		return _task(a)
	case *iface.Message:
		if !a.router.HasRoute(uint16(m.GetId())) {
			break
		}
		_, err := a.execHandler(m)
		return err
	case *iface.SyncMessage:
		if !a.router.HasRoute(uint16(m.GetId())) {
			break
		}
		data, err := a.execHandler(m.Message)
		m.Response(data, err)
		return err
	}

	if err := a.actor.OnMessage(a, msg); err != nil {
		return err
	}

	return nil
}

func (a *baseActorContext) execHandler(msg *iface.Message) (data []byte, err error) {
	if a.router == nil {
		err = fmt.Errorf("router is nil")
		return
	}
	if msg == nil {
		err = fmt.Errorf("msg is nil")
		return
	}
	return a.router.Handle(a, uint16(msg.GetId()), msg.GetData())
}

func (a *baseActorContext) Actor() iface.IActor {
	return a.actor
}

func (a *baseActorContext) GetSerializer() serializer.ISerializer {
	if a.system != nil {
		return a.system.GetSerializer()
	}
	return serializer.Json // 默认序列化器
}

func (a *baseActorContext) Exit() {
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

	ser := a.system.GetSerializer()

	// 序列化 request
	requestData, err := ser.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	var message *iface.Message
	switch m := request.(type) {
	case *iface.Message:
		message = m
	default:
		// 构建消息
		message = &iface.Message{
			To:   to,
			From: a.pid,
			Id:   uint32(msgId),
			Data: requestData,
		}
	}

	n := a.system.GetNode()
	if n == nil {
		return a.system.Send(message)
	} else {
		// 调用 node.Send
		return n.GetRemote().Send(message)
	}
}

func (a *baseActorContext) Request(to *iface.Pid, msgId uint16, request interface{}, reply interface{}) error {
	if to == nil {
		return fmt.Errorf("target pid is nil")
	}
	if request == nil {
		return fmt.Errorf("request is nil")
	}
	if reply == nil {
		return fmt.Errorf("reply is nil")
	}

	if a.system == nil {
		return fmt.Errorf("system is not initialized")
	}

	ser := a.system.GetSerializer()

	// 序列化 request
	requestData, err := ser.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	var message *iface.Message
	switch m := request.(type) {
	case *iface.Message:
		message = m
	default:
		// 构建消息
		message = &iface.Message{
			To:   to,
			From: a.pid,
			Id:   uint32(msgId),
			Data: requestData,
		}
	}

	timeout := 5 * time.Second
	var response *iface.RespondMessage
	n := a.system.GetNode()
	if n == nil {
		response = a.system.Request(message, timeout)
	} else {
		// 调用 node.Request（使用默认超时时间 5 秒）
		response = n.GetRemote().Request(message, timeout)
	}

	// 检查响应错误
	if response.GetError() != "" {
		return fmt.Errorf("request failed: %s", response.GetError())
	}

	// 反序列化响应到 reply
	if len(response.GetData()) > 0 {
		if err = ser.Unmarshal(response.GetData(), reply); err != nil {
			return fmt.Errorf("unmarshal reply failed: %w", err)
		}
	}

	return nil
}

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

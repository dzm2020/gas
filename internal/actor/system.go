/**
 * @Author: dingQingHui
 * @Description:
 * @File: system
 * @Version: 1.0.0
 * @Date: 2023/12/7 14:54
 */

package actor

import (
	"errors"
	"gas/internal/iface"
	"gas/pkg/utils/serializer"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/maputil"
)

// System Actor 系统，管理所有进程和消息传递
type System struct {
	uniqId      atomic.Uint64
	nameDict    *maputil.ConcurrentMap[string, iface.IProcess]
	processDict *maputil.ConcurrentMap[uint64, iface.IProcess]
	serializer  serializer.ISerializer
	node        iface.INode
}

// NewSystem 创建新的 Actor 系统
func NewSystem() *System {
	return &System{
		uniqId:      atomic.Uint64{},
		nameDict:    maputil.NewConcurrentMap[string, iface.IProcess](10),
		processDict: maputil.NewConcurrentMap[uint64, iface.IProcess](10),
		serializer:  serializer.Json,
	}
}

// SetNode 设置节点实例
func (s *System) SetNode(n iface.INode) {
	s.node = n
}

// GetNode 获取节点实例
func (s *System) GetNode() iface.INode {
	return s.node
}

// GetSerializer 获取序列化器
func (s *System) GetSerializer() serializer.ISerializer {
	return s.serializer
}

// SetSerializer 设置序列化器
func (s *System) SetSerializer(ser serializer.ISerializer) {
	s.serializer = ser
}

// newPid 创建新的进程ID
func (s *System) newPid() *iface.Pid {
	pid := &iface.Pid{
		ServiceId: s.uniqId.Add(1),
	}
	if s.node != nil {
		pid.NodeId = s.node.GetId()
	}
	return pid
}

// Spawn 创建新的 actor 进程
func (s *System) Spawn(actor iface.IActor, options ...Option) (*iface.Pid, iface.IProcess) {
	opts := loadOptions(options...)
	pid := s.newPid()
	pid.Name = opts.Name

	context := newBaseActorContext(pid, actor, opts.Middlewares, opts.Router, s)
	mailBox := NewMailbox()
	mailBox.RegisterHandlers(context, NewDefaultDispatcher(50))
	process := NewProcess(context, mailBox)

	s.registerProcess(pid, process)

	_ = process.PushTask(func(ctx iface.IContext) error {
		return ctx.Actor().OnInit(ctx, opts.Params)
	})

	return pid, process
}

// GetProcess 根据 Pid 获取进程
func (s *System) GetProcess(pid *iface.Pid) iface.IProcess {
	if pid == nil {
		return nil
	}
	name := pid.GetName()
	if name != "" {
		p, _ := s.nameDict.Get(name)
		return p
	}
	serviceId := pid.GetServiceId()
	if serviceId > 0 {
		p, _ := s.processDict.Get(serviceId)
		return p
	}
	return nil
}

// GetProcessById 根据 ID 获取进程
func (s *System) GetProcessById(id uint64) iface.IProcess {
	process, _ := s.processDict.Get(id)
	return process
}

// GetProcessByName 根据名称获取进程
func (s *System) GetProcessByName(name string) iface.IProcess {
	process, _ := s.nameDict.Get(name)
	return process
}

// Send 发送消息到指定进程
func (s *System) Send(message *iface.Message) error {
	process := s.GetProcess(message.GetTo())
	if process == nil {
		return errors.New("process not found")
	}
	return process.PushMessage(message)
}

// Request 发送消息到指定进程
func (s *System) Request(message *iface.Message, timeout time.Duration) *iface.RespondMessage {
	waiter := newChanWaiter[*iface.RespondMessage](timeout)

	msg := &SyncMessage{Message: message}
	msg.SetResponse(func(message *iface.RespondMessage) {
		waiter.Done(message)
	})

	process := s.GetProcess(msg.GetTo())
	if process == nil {
		return iface.NewErrorResponse("process not found")
	}
	if err := process.PushMessage(msg); err != nil {
		return iface.NewErrorResponse(err.Error())
	}

	res, err := waiter.Wait()
	if err != nil {
		return iface.NewErrorResponse(err.Error())
	}

	return res
}

// registerProcess 注册进程
func (s *System) registerProcess(pid *iface.Pid, process iface.IProcess) {
	s.processDict.Set(pid.GetServiceId(), process)
	if pid.GetName() != "" {
		s.nameDict.Set(pid.GetName(), process)
	}
}

// unregisterProcess 注销进程
func (s *System) unregisterProcess(pid *iface.Pid) {
	s.processDict.Delete(pid.GetServiceId())
	if pid.GetName() != "" {
		s.nameDict.Delete(pid.GetName())
	}
}

// GetAllProcesses 获取所有进程
func (s *System) GetAllProcesses() []iface.IProcess {
	var processes []iface.IProcess
	// 使用 nameDict 来获取所有进程（避免重复）
	// 因为同一个进程可能同时存在于 processDict 和 nameDict 中
	seen := make(map[uint64]bool)

	// 从 processDict 获取所有进程
	s.processDict.Range(func(key uint64, value iface.IProcess) bool {
		if !seen[key] {
			processes = append(processes, value)
			seen[key] = true
		}
		return true
	})

	return processes
}

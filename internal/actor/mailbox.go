package actor

import (
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib"
	"runtime"
	"sync/atomic"

	"go.uber.org/zap"
)

const (
	idle int32 = iota
	running
)

type IMailbox interface {
	PostMessage(msg interface{}) error
	RegisterHandlers(invoker iface.IMessageInvoker, dispatcher IDispatcher)
	IsEmpty() bool
}

var _ IMailbox = &Mailbox{}

type Mailbox struct {
	invoker      iface.IMessageInvoker
	queue        *lib.Mpsc
	dispatch     IDispatcher
	dispatchStat atomic.Int32
}

func NewMailbox() *Mailbox {
	m := &Mailbox{
		queue: lib.NewMpsc(),
	}
	return m
}

func (mb *Mailbox) RegisterHandlers(invoker iface.IMessageInvoker, dispatcher IDispatcher) {
	mb.invoker = invoker
	mb.dispatch = dispatcher
}

func (mb *Mailbox) PostMessage(msg interface{}) error {
	if msg == nil {
		return nil
	}
	mb.queue.Push(msg)
	return mb.schedule()
}

// schedule 调度消息处理
// 使用 CAS 操作确保同一时间只有一个 goroutine 在处理消息队列
// 如果已经有 goroutine 在处理，则直接返回
func (mb *Mailbox) schedule() error {
	// 尝试将状态从 idle 切换到 running
	// 如果失败说明已经有 goroutine 在处理，直接返回
	if !mb.dispatchStat.CompareAndSwap(idle, running) {
		return nil
	}
	// 调度消息处理函数，如果发生 panic 则记录日志
	if err := mb.dispatch.Schedule(mb.process, func(err interface{}) {
		glog.Errorf("Mailbox dispatch schedule panic:%+v stack:%+v", err, zap.Stack("stack"))
	}); err != nil {
		glog.Errorf("Mailbox dispatch schedule error:%v", err)
		return err
	}
	return nil
}

// process 消息处理入口函数
// 使用 defer 确保无论是否发生 panic，状态都能被重置为 idle
// 这样其他 goroutine 可以继续处理队列中的消息
func (mb *Mailbox) process() {
	defer func() {
		// 确保无论是否发生 panic，状态都能被重置
		// 使用 CompareAndSwap 确保原子性
		mb.dispatchStat.CompareAndSwap(running, idle)
	}()
	mb.run()
}

// run 执行消息处理循环
// 从队列中取出消息并处理，每处理一定数量后让出 CPU，避免长时间占用
func (mb *Mailbox) run() {
	throughput := mb.dispatch.Throughput()
	var processed int

	for {
		// 检查队列是否为空
		if mb.queue.Empty() {
			return
		}

		// 每处理一定数量的消息后让出 CPU，避免长时间占用导致其他 goroutine 饥饿
		// 这样可以提高系统的响应性和公平性
		if processed >= throughput {
			processed = 0
			runtime.Gosched()
			continue
		}

		processed++
		msg := mb.queue.Pop()
		if msg == nil {
			return
		}
		// 处理消息，错误已由 invoker 处理，这里只记录日志
		if err := mb.invokerMessage(msg); err != nil {
			glog.Error("处理消息失败", zap.Error(err))
		}
	}
}

func (mb *Mailbox) invokerMessage(msg interface{}) error {
	return mb.invoker.InvokerMessage(msg)
}

// IsEmpty 检查 mailbox 队列是否为空
func (mb *Mailbox) IsEmpty() bool {
	return mb.queue.Empty()
}

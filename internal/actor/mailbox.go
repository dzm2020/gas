package actor

import (
	"gas/internal/iface"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
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

func (mb *Mailbox) schedule() error {
	if !mb.dispatchStat.CompareAndSwap(idle, running) {
		return nil
	}
	if err := mb.dispatch.Schedule(mb.process, func(err interface{}) {
		glog.Errorf("Mailbox dispatch schedule panic:%+v stack:%+v", err, zap.Stack("stack"))
	}); err != nil {
		glog.Errorf("Mailbox dispatch schedule error:%v", err)
		return err
	}
	return nil
}

func (mb *Mailbox) process() {
	defer func() {
		// 确保无论是否发生 panic，状态都能被重置
		mb.dispatchStat.CompareAndSwap(running, idle)
	}()
	mb.run()
}

func (mb *Mailbox) run() {
	throughput := mb.dispatch.Throughput()
	var processed int

	for {
		if mb.queue.Empty() {
			return
		}

		// 每处理一定数量的消息后让出 CPU
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
		_ = mb.invokerMessage(msg)
	}
}

func (mb *Mailbox) invokerMessage(msg interface{}) error {
	return mb.invoker.InvokerMessage(msg)
}

// IsEmpty 检查 mailbox 队列是否为空
func (mb *Mailbox) IsEmpty() bool {
	return mb.queue.Empty()
}

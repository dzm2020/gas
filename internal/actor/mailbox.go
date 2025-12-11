package actor

import (
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
	RegisterHandlers(invoker IMessageInvoker, dispatcher IDispatcher)
	IsEmpty() bool
}

var _ IMailbox = &Mailbox{}

type Mailbox struct {
	invoker      IMessageInvoker
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

func (m *Mailbox) RegisterHandlers(invoker IMessageInvoker, dispatcher IDispatcher) {
	m.invoker = invoker
	m.dispatch = dispatcher
}

func (m *Mailbox) PostMessage(msg interface{}) error {
	if msg == nil {
		return nil
	}
	m.queue.Push(msg)
	return m.schedule()
}

func (m *Mailbox) schedule() error {
	if !m.dispatchStat.CompareAndSwap(idle, running) {
		return nil
	}
	if err := m.dispatch.Schedule(m.process, func(err interface{}) {
		glog.Errorf("Mailbox dispatch schedule panic:%+v stack:%+v", err, zap.Stack("stack"))
	}); err != nil {
		glog.Errorf("Mailbox dispatch schedule error:%v", err)
		return err
	}
	return nil
}

func (m *Mailbox) process() {
	defer func() {
		// 确保无论是否发生 panic，状态都能被重置
		m.dispatchStat.CompareAndSwap(running, idle)
	}()
	m.run()
}

func (m *Mailbox) run() {
	throughput := m.dispatch.Throughput()
	var processed int

	for {
		if m.queue.Empty() {
			return
		}

		// 每处理一定数量的消息后让出 CPU
		if processed >= throughput {
			processed = 0
			runtime.Gosched()
			continue
		}

		processed++
		msg := m.queue.Pop()
		if msg == nil {
			return
		}
		_ = m.invokerMessage(msg)
	}
}

func (m *Mailbox) invokerMessage(msg interface{}) error {
	return m.invoker.InvokerMessage(msg)
}

// IsEmpty 检查 mailbox 队列是否为空
func (m *Mailbox) IsEmpty() bool {
	return m.queue.Empty()
}

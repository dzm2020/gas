package actor

import (
	"gas/pkg/utils/glog"
	"gas/pkg/utils/mpsc"
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
	queue        *mpsc.Queue
	dispatch     IDispatcher
	dispatchStat atomic.Int32
}

func NewMailbox() *Mailbox {
	m := &Mailbox{
		queue: mpsc.NewQueue(),
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
	m.run()
	m.dispatchStat.CompareAndSwap(running, idle)
}

func (m *Mailbox) run() {
	throughput := m.dispatch.Throughput()
	var i int
	for {
		if m.queue.Empty() {
			return
		}
		if i > throughput {
			i = 0
			runtime.Gosched()
			continue
		}
		i++
		msg := m.queue.Pop()
		if msg != nil {
			_ = m.invokerMessage(msg)
		} else {
			return
		}
	}
}

func (m *Mailbox) invokerMessage(msg interface{}) error {
	return m.invoker.InvokerMessage(msg)
}

// IsEmpty 检查 mailbox 队列是否为空
func (m *Mailbox) IsEmpty() bool {
	return m.queue.Empty()
}

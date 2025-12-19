package actor

import (
	"gas/internal/iface"
	"sync/atomic"
)

// NewProcess 创建新的进程实例
func NewProcess(ctx *actorContext, mailbox IMailbox) *Process {
	process := &Process{
		mailbox: mailbox,
		ctx:     ctx,
	}
	return process
}

var _ iface.IProcess = (*Process)(nil)

type Process struct {
	mailbox  IMailbox
	ctx      *actorContext
	shutdown atomic.Bool
}

func (p *Process) Context() iface.IContext {
	return p.ctx
}

func (p *Process) checkShutdown() error {
	if p.shutdown.Load() {
		return ErrProcessExiting
	}
	return nil
}

func (p *Process) PostMessage(message iface.IMessage) error {
	if err := p.checkShutdown(); err != nil {
		return err
	}
	if err := message.Validate(); err != nil {
		return err
	}
	return p.mailbox.PostMessage(message)
}

func (p *Process) Shutdown() error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil // 已经在退出中
	}

	msg := iface.NewTaskMessage(func(ctx iface.IContext) error {
		p.ctx.exit()
		return nil
	})

	return p.mailbox.PostMessage(msg)
}

package actor

import (
	"errors"
	"gas/internal/iface"
	"sync/atomic"
	"time"
)

func NewProcess(ctx iface.IContext, mailbox IMailbox) iface.IProcess {
	process := &Process{
		mailbox: mailbox,
		ctx:     ctx,
	}
	return process
}

var _ iface.IProcess = (*Process)(nil)

type Process struct {
	mailbox IMailbox
	ctx     iface.IContext
	isExit  atomic.Bool
}

func (p *Process) Context() iface.IContext {
	return p.ctx
}

func (p *Process) PushTask(task iface.Task) error {
	if task == nil {
		return nil
	}
	return p.mailbox.PostMessage(&TaskMessage{
		task: task,
	})
}

func (p *Process) PushTaskAndWait(timeout time.Duration, task iface.Task) error {
	if task == nil {
		return errors.New("task is nil")
	}

	waiter := newChanWaiter[error](timeout)

	syncTask := func(ctx iface.IContext) error {
		e := task(ctx)
		waiter.Done(e)
		return e
	}

	if err := p.mailbox.PostMessage(&TaskMessage{task: syncTask}); err != nil {
		return err
	}
	_, err := waiter.Wait()
	return err
}

func (p *Process) PushMessage(message interface{}) error {
	return p.mailbox.PostMessage(message)
}

func (p *Process) Exit() error {
	if !p.isExit.CompareAndSwap(false, true) {
		return nil
	}
	return p.PushTask(func(ctx iface.IContext) error {
		ctx.Exit()
		return nil
	})
}

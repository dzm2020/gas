package actor

import (
	"errors"
	"sync/atomic"
	"time"
)

type IProcess interface {
	Context() IContext
	PushTask(f Task) error
	PushTaskAndWait(timeout time.Duration, task Task) error
}

func NewProcess(ctx IContext, mailbox IMailbox) IProcess {
	process := &Process{
		mailbox: mailbox,
		ctx:     ctx,
	}
	return process
}

var _ IProcess = (*Process)(nil)

type Process struct {
	mailbox IMailbox
	ctx     IContext
	isExit  atomic.Bool
}

func (p *Process) Context() IContext {
	return p.ctx
}

func (p *Process) PushTask(task Task) error {
	if task == nil {
		return nil
	}
	return p.mailbox.PostMessage(&TaskMessage{
		task: task,
	})
}

func (p *Process) PushTaskAndWait(timeout time.Duration, task Task) error {
	if task == nil {
		return errors.New("task is nil")
	}

	waiter := newChanWaiter(timeout)

	syncTask := func(ctx IContext) error {
		e := task(ctx)
		waiter.Done(e)
		return e
	}

	if err := p.mailbox.PostMessage(&TaskMessage{task: syncTask}); err != nil {
		return err
	}

	return waiter.Wait()
}

func (p *Process) Exit() error {
	if !p.isExit.CompareAndSwap(false, true) {
		return nil
	}
	return p.PushTask(func(ctx IContext) error {
		ctx.Exit()
		return nil
	})
}

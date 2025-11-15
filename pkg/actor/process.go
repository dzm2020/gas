package actor

type IProcess interface {
	Context() IContext
	PushTask(f Task) error
}

func NewProcess(ctx IContext, mailbox IMailbox) IProcess {
	process := &Process{
		mailbox: mailbox,
		ctx:     ctx,
	}
	processDict.Set(ctx.ID(), process)
	return process
}

var _ IProcess = (*Process)(nil)

type Process struct {
	mailbox IMailbox
	ctx     IContext
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

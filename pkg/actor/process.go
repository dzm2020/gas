package actor

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

func (p *Process) Post(name string, args ...interface{}) error {
	m := &Message{
		Name: name,
		Args: args,
	}
	return p.mailbox.PostMessage(m)
}

func (p *Process) SubmitTask(f func() error) error {
	return p.mailbox.PostMessage(&AsyncCallMessage{
		f: f,
	})
}

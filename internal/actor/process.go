package actor

import (
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/lib/stopper"
)

// newProcess 创建新的进程实例
func newProcess(mailbox IMailbox) *Process {
	process := &Process{
		mailbox: mailbox,
	}
	return process
}

var _ iface.IProcess = (*Process)(nil)

type Process struct {
	mailbox IMailbox
	stopper.Stopper
}

func (p *Process) PostMessage(message iface.IMessage) error {
	if p.IsStop() {
		return ErrProcessExiting
	}
	if err := message.Validate(); err != nil {
		return err
	}
	return p.mailbox.PostMessage(message)
}

// Shutdown 优雅关闭进程
func (p *Process) Shutdown() error {
	if !p.Stop() {
		return nil
	}
	// 创建一个退出任务，通过 mailbox 发送，确保在消息处理完成后才执行退出
	msg := iface.NewTaskMessage(func(ctx iface.IContext) error {
		_ = ctx.System().Unregister(ctx)
		return ctx.Actor().OnStop(ctx)
	})
	return p.mailbox.PostMessage(msg)
}

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

// Shutdown 优雅关闭进程
// 通过发送退出任务到 mailbox 来确保在消息处理完成后才退出
// 使用 CAS 操作确保只执行一次关闭操作
func (p *Process) Shutdown() error {
	// 尝试将 shutdown 状态从 false 切换到 true
	// 如果失败说明已经在关闭中，直接返回
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil // 已经在退出中
	}

	// 创建一个退出任务，通过 mailbox 发送，确保在消息处理完成后才执行退出
	msg := iface.NewTaskMessage(func(ctx iface.IContext) error {
		return p.ctx.exit()
	})

	return p.mailbox.PostMessage(msg)
}

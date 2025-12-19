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

// Process Actor 进程实现，管理消息队列和执行
type Process struct {
	mailbox  IMailbox
	ctx      *actorContext
	shutdown atomic.Bool
}

// Context 获取进程的上下文
func (p *Process) Context() iface.IContext {
	return p.ctx
}

// checkShutdown 检查进程是否正在退出
func (p *Process) checkShutdown() error {
	if p.shutdown.Load() {
		return ErrProcessExiting
	}
	return nil
}

// validateMessage 验证消息是否合法
func (p *Process) validateMessage(message interface{}) error {
	if message == nil {
		return ErrMessageIsNil
	}
	// 使用 IMessageValidator 接口验证消息
	if validator, ok := message.(iface.IMessageValidator); ok {
		if err := validator.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Process) Input(message interface{}) error {
	if err := p.checkShutdown(); err != nil {
		return err
	}

	if err := p.validateMessage(message); err != nil {
		return err
	}
	return p.mailbox.PostMessage(message)
}

// Shutdown 退出进程，清理资源并等待所有任务完成
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

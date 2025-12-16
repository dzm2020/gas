package actor

import (
	"gas/internal/errs"
	"gas/internal/iface"
	"sync/atomic"
	"time"
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
		return errs.ErrProcessExiting
	}
	return nil
}

// validateMessage 验证消息是否合法
func (p *Process) validateMessage(message interface{}) error {
	if message == nil {
		return errs.ErrMessageIsNil
	}
	// 使用 IMessageValidator 接口验证消息
	if validator, ok := message.(iface.IMessageValidator); ok {
		if err := validator.Validate(); err != nil {
			return errs.ErrInvalidMessage(err)
		}
	}
	return nil
}

func (p *Process) postMessage(message interface{}) error {
	if err := p.checkShutdown(); err != nil {
		return err
	}
	// 验证消息合法性
	if err := p.validateMessage(message); err != nil {
		return err
	}
	return p.mailbox.PostMessage(message)
}

// PushTask 推送任务到进程的消息队列（异步执行）
func (p *Process) PushTask(task iface.Task) error {
	if task == nil {
		return nil
	}
	taskMsg := &iface.TaskMessage{
		Task: task,
	}
	return p.postMessage(taskMsg)
}

// PushTaskAndWait 推送任务并等待执行完成（同步执行）
func (p *Process) PushTaskAndWait(timeout time.Duration, task iface.Task) error {
	if task == nil {
		return errs.ErrTaskIsNil
	}
	if err := p.checkShutdown(); err != nil {
		return err
	}
	return p.pushTaskAndWait(timeout, task)
}

// pushTaskAndWait 推送任务并等待完成（内部方法，绕过退出检查）
// 注意：此方法直接调用 mailbox.PostMessage，不经过 checkShutdown 检查
// 用于确保退出任务等关键任务能够执行
func (p *Process) pushTaskAndWait(timeout time.Duration, task iface.Task) error {
	if task == nil {
		return errs.ErrTaskIsNil
	}

	waiter := newChanWaiter[error](timeout)

	syncTask := func(ctx iface.IContext) error {
		var err error
		if task != nil {
			err = task(ctx)
		}
		waiter.Done(err)
		return err
	}

	// 直接调用 mailbox.PostMessage，绕过 checkShutdown 检查
	if err := p.mailbox.PostMessage(&iface.TaskMessage{Task: syncTask}); err != nil {
		waiter.Done(err)
		return err
	}

	_, err := waiter.Wait()
	return err
}

// Send 发送消息到进程
func (p *Process) Send(message *iface.ActorMessage) error {
	message.Async = true
	return p.postMessage(message)
}

// Call 发送同步请求消息并等待响应
func (p *Process) Call(message *iface.ActorMessage, timeout time.Duration) *iface.Response {
	message.Async = false
	waiter := newChanWaiter[*iface.Response](timeout)
	message.SetResponse(func(response *iface.Response) {
		waiter.Done(response)
	})
	if err := p.postMessage(message); err != nil {
		return iface.NewErrorResponse(err)
	}
	res, err := waiter.Wait()
	if err != nil {
		return iface.NewErrorResponse(err)
	}
	return res
}

// Shutdown 退出进程，清理资源并等待所有任务完成
func (p *Process) Shutdown() error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil // 已经在退出中
	}
	// 设置退出标志后，使用 pushTaskAndWait 推送退出任务
	// pushTaskAndWait 直接调用 mailbox.PostMessage，绕过 checkShutdown 检查，确保退出任务能够执行
	return p.pushTaskAndWait(time.Second, func(ctx iface.IContext) error {
		p.ctx.exit()
		return nil
	})
}

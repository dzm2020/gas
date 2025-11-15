// Package actor
// @Description:

package actor

type Task func(ctx IContext) error

type TaskMessage struct {
	task Task
}

type IMessageInvoker interface {
	InvokerMessage(message interface{}) error
}

type IContext interface {
	ID() uint64
	Actor() IActor
}

func newBaseActorContext(id uint64, actor IActor, middlerWares []TaskMiddleware) *baseActorContext {
	ctx := &baseActorContext{
		id:           id,
		actor:        actor,
		middlerWares: middlerWares,
	}
	return ctx
}

type baseActorContext struct {
	id           uint64
	actor        IActor
	middlerWares []TaskMiddleware
}

func (a *baseActorContext) ID() uint64 {
	return a.id
}
func (a *baseActorContext) InvokerMessage(msg interface{}) error {
	switch m := msg.(type) {
	case *TaskMessage:
		_task := chain(a.middlerWares, m.task)
		return _task(a)
	}
	return nil
}

func (a *baseActorContext) Actor() IActor {
	return a.actor
}

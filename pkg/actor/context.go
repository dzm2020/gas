// Package actor
// @Description:

package actor

import "errors"

func newBaseActorContext(id uint64, actor IActor, router IRouter) *baseActorContext {
	ctx := &baseActorContext{
		id:     id,
		router: router,
		actor:  actor,
	}
	return ctx
}

type baseActorContext struct {
	id     uint64
	actor  IActor
	router IRouter
	mbm    interface{}
}

func (a *baseActorContext) ID() uint64 {
	return a.id
}
func (a *baseActorContext) InvokerMessage(msg interface{}) error {

	m, ok := msg.(*Message)
	if !ok {
		return errors.New("invalid message type")
	}
	args := []interface{}{a.actor, a}
	args = append(args, m.Args...)
	a.router.Handle(m.Name, args...)

	return nil
}

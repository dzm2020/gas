package actor

type (
	Producer func() IActor

	IMessageInvoker interface {
		InvokerMessage(message interface{}) error
	}

	IDispatcher interface {
		Schedule(f func(), recoverFun func(err interface{})) error
		Throughput() int
	}
	IProcess interface {
		Context() IContext
		Post(name string, args ...interface{}) error
		SubmitTask(f func() error) error
	}
	IMailbox interface {
		PostMessage(msg interface{}) error
		RegisterHandlers(invoker IMessageInvoker, dispatcher IDispatcher)
	}

	IContext interface {
		ID() uint64
	}
	IActor interface {
		OnInit(ctx IContext, params []interface{}) error
		OnMessage(ctx IContext, msg interface{}) error
		OnStop(ctx IContext) error
	}

	IRouter interface {
		Handle(name string, args ...interface{}) []interface{}
	}
)

var _ IActor = (*Actor)(nil)

type Actor struct {
}

func (a *Actor) OnInit(ctx IContext, params []interface{}) error {
	return nil
}

func (a *Actor) OnMessage(ctx IContext, msg interface{}) error {
	return nil
}

func (a *Actor) OnStop(ctx IContext) error {
	return nil
}

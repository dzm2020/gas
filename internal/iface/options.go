package iface

type Option func(*Options)

type Options struct {
	Params      []interface{}
	Middlewares []TaskMiddleware
	Name        string
	Router      IRouter
}

func WithParams(params []interface{}) Option {
	return func(op *Options) {
		op.Params = params
	}
}

func WithMiddlewares(middlewares []TaskMiddleware) Option {
	return func(op *Options) {
		op.Middlewares = middlewares
	}
}

func WithName(name string) Option {
	return func(op *Options) {
		op.Name = name
	}
}

func WithRouter(r IRouter) Option {
	return func(op *Options) {
		op.Router = r
	}
}

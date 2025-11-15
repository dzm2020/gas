package actor

type Option func(*Options)

type Options struct {
	Params      []interface{}
	Middlewares []TaskMiddleware
	Name        string
}

func loadOptions(options ...Option) *Options {
	opts := &Options{}
	for _, option := range options {
		option(opts)
	}
	return opts
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

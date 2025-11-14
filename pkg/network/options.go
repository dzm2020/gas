package network

import (
	"github.com/panjf2000/gnet/v2"
)

type Option func(*Options)

func loadOptions(options ...Option) *Options {
	opts := defaultOptions()
	for _, option := range options {
		option(opts)
	}
	return opts
}

func defaultOptions() *Options {
	opts := &Options{}
	return opts
}

type Options struct {
	opts           []gnet.Option
	sessionFactory func() ISession
}

func WithSessionFactory(factory func() ISession) Option {
	return func(op *Options) {
		op.sessionFactory = factory
	}
}

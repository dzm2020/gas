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
	opts    []gnet.Option
	handler IHandler
}

func WithHandler(handler IHandler) Option {
	return func(op *Options) {
		op.handler = handler
	}
}

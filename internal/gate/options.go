package gate

import (
	"time"
)

type Option func(*Options)

type Options struct {
	ProtoAddr   string
	MaxConn     int32
	GracePeriod time.Duration
}

func defaultOptions() *Options {
	return &Options{
		ProtoAddr:   "tcp://127.0.0.1:9000",
		MaxConn:     100000,
		GracePeriod: 5 * time.Second,
	}
}

func loadOptions(opts ...Option) *Options {
	options := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}
	if options.ProtoAddr == "" {
		options.ProtoAddr = "tcp://127.0.0.1:9000"
	}
	if options.MaxConn <= 0 {
		options.MaxConn = 100000
	}
	if options.GracePeriod <= 0 {
		options.GracePeriod = 5 * time.Second
	}
	return options
}

func WithProtoAddr(addr string) Option {
	return func(o *Options) {
		o.ProtoAddr = addr
	}
}

func WithMaxConn(MaxConn int32) Option {
	return func(o *Options) {
		o.MaxConn = MaxConn
	}
}

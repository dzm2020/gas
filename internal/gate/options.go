package gate

import "gas/internal/protocol"

type Option func(*Options)

type Options struct {
	ProtoAddr string
	Router    IRouter
	codec     protocol.ICodec
}

func defaultOptions() *Options {
	return &Options{
		ProtoAddr: "tcp://127.0.0.1:9000",
		Router:    NewProtoMessageRouter(),
	}
}

func loadOptions(opts ...Option) *Options {
	options := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}
	return options
}

func WithProtoAddr(addr string) Option {
	return func(o *Options) {
		o.ProtoAddr = addr
	}
}

func WithRouter(router IRouter) Option {
	return func(o *Options) {
		o.Router = router
	}
}

func WithCodec(codec protocol.ICodec) Option {
	return func(o *Options) {
		o.codec = codec
	}
}

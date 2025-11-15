package gate

import (
	"time"

	"gas/internal/gate/codec"
)

type Option func(*Options)

type Options struct {
	ProtoAddr      string
	Router         IRouter
	Codec          codec.ICodec
	MaxConn        int32
	ReadBufferSize uint32
	GracePeriod    time.Duration
}

func defaultOptions() *Options {
	return &Options{
		ProtoAddr:      "tcp://127.0.0.1:9000",
		Router:         NewProtoMessageRouter(),
		ReadBufferSize: 1024 * 4,
		Codec:          codec.New(),
		MaxConn:        100000,
		GracePeriod:    5 * time.Second,
	}
}

func loadOptions(opts ...Option) *Options {
	options := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}
	if options.Router == nil {
		options.Router = NewProtoMessageRouter()
	}
	if options.Codec == nil {
		options.Codec = codec.New()
	}
	if options.ProtoAddr == "" {
		options.ProtoAddr = "tcp://127.0.0.1:9000"
	}
	if options.ReadBufferSize == 0 {
		options.ReadBufferSize = 1024 * 4
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

func WithRouter(router IRouter) Option {
	return func(o *Options) {
		o.Router = router
	}
}

func WithCodec(codec codec.ICodec) Option {
	return func(o *Options) {
		o.Codec = codec
	}
}

func WithMaxConn(MaxConn int32) Option {
	return func(o *Options) {
		o.MaxConn = MaxConn
	}
}
func WithReadBufferSize(ReadBufferSize uint32) Option {
	return func(o *Options) {
		o.ReadBufferSize = ReadBufferSize
	}
}

func WithGracePeriod(d time.Duration) Option {
	return func(o *Options) {
		o.GracePeriod = d
	}
}

package network

import (
	"time"
)

type Option func(*Options)
type Options struct {
	handler      IHandler      // 业务回调
	codec        ICodec        // 协议编解码器
	keepAlive    time.Duration // 连接超时时间（0表示不检测超时）
	sendChanSize int           // 发送队列缓冲大小
	readBufSize  int           // 读缓冲区大小
	maxConn      int
}

func loadOptions(options ...Option) *Options {
	opts := &Options{
		sendChanSize: defaultSendChanBuf,
		readBufSize:  defaultTCPReadBuf,
		keepAlive:    defaultKeepAlive,
		maxConn:      10000,
	}
	for _, option := range options {
		option(opts)
	}
	return opts
}

// WithSendChanSize 设置发送队列缓冲大小
func WithSendChanSize(size int) Option {
	return func(opts *Options) {
		opts.sendChanSize = size
	}
}

// WithReadBufSize 设置读缓冲区大小
func WithReadBufSize(size int) Option {
	return func(opts *Options) {
		opts.readBufSize = size
	}
}

// WithHandler 设置业务回调处理器
func WithHandler(handler IHandler) Option {
	return func(opts *Options) {
		if handler == nil {
			return
		}
		opts.handler = handler
	}
}

// WithCodec 设置协议编解码器
func WithCodec(codec ICodec) Option {
	return func(opts *Options) {
		if codec == nil {
			return
		}
		opts.codec = codec
	}
}

// WithKeepAlive 设置连接超时时间
func WithKeepAlive(keepAlive time.Duration) Option {
	return func(opts *Options) {
		if keepAlive <= 0 {
			return
		}
		opts.keepAlive = keepAlive
	}
}

// WithMaxConn 设置最大连接数
func WithMaxConn(maxConn int) Option {
	return func(opts *Options) {
		if maxConn <= 0 {
			return
		}
		opts.maxConn = maxConn
	}
}

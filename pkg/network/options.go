package network

import (
	"net/http"
	"time"
)

type Option func(*Options)
type Options struct {
	Handler        IHandler                      // 业务回调
	Codec          ICodec                        // 协议编解码器
	HeartTimeout   time.Duration                 // 心跳超时（0表示不检测超时）
	SendBufferSize int                           // 发送队列缓冲大小
	ReadBufSize    int                           // 读缓冲区大小
	TLSCertFile    string                        // TLS 证书文件路径
	TLSKeyFile     string                        // TLS 私钥文件路径
	ReusePort      bool                          // 是否启用 SO_REUSEPORT（仅 Linux 支持）
	ReuseAddr      bool                          // 是否启用 SO_REUSEADDR
	SendChanSize   int
	UdpRcvChanSize int
	CheckOrigin    func(r *http.Request) bool    // WebSocket Origin 检查函数（nil 表示不检查）
}

func loadOptions(options ...Option) *Options {
	opts := &Options{
		Handler:        &EmptyHandler{},
		Codec:          &EmptyCodec{},
		HeartTimeout:   5 * time.Second,
		SendBufferSize: 1024 * 4,
		ReadBufSize:    1024 * 4,
		UdpRcvChanSize: 1024,
		SendChanSize:   1024,
		ReuseAddr:      true,
		ReusePort:      false,
	}
	for _, option := range options {
		option(opts)
	}
	return opts
}

// WithSendBufferSize 设置发送队列缓冲大小
func WithSendBufferSize(size int) Option {
	return func(opts *Options) {
		opts.SendBufferSize = size
	}
}

// WithReadBufSize 设置读缓冲区大小
func WithReadBufSize(size int) Option {
	return func(opts *Options) {
		opts.ReadBufSize = size
	}
}

// WithHandler 设置业务回调处理器
func WithHandler(handler IHandler) Option {
	return func(opts *Options) {
		if handler == nil {
			return
		}
		opts.Handler = handler
	}
}

// WithCodec 设置协议编解码器
func WithCodec(codec ICodec) Option {
	return func(opts *Options) {
		if codec == nil {
			return
		}
		opts.Codec = codec
	}
}

// WithKeepAlive 设置连接超时时间
func WithKeepAlive(heartInterval time.Duration) Option {
	return func(opts *Options) {
		if heartInterval <= 0 {
			return
		}
		opts.HeartTimeout = heartInterval
	}
}

// WithTLS 设置 TLS 证书和私钥文件路径
func WithTLS(certFile, keyFile string) Option {
	return func(opts *Options) {
		opts.TLSCertFile = certFile
		opts.TLSKeyFile = keyFile
	}
}

// WithReusePort 设置是否启用 SO_REUSEPORT（仅 Linux 支持）
func WithReusePort(enable bool) Option {
	return func(opts *Options) {
		opts.ReusePort = enable
	}
}

// WithReuseAddr 设置是否启用 SO_REUSEADDR
func WithReuseAddr(enable bool) Option {
	return func(opts *Options) {
		opts.ReuseAddr = enable
	}
}

func WithSendChanSize(sendChanSize int) Option {
	return func(opts *Options) {
		opts.SendChanSize = sendChanSize
	}
}

func WithUdpRcvChanSize(udpRcvChanSize int) Option {
	return func(opts *Options) {
		opts.UdpRcvChanSize = udpRcvChanSize
	}
}

// WithCheckOrigin 设置 WebSocket Origin 检查函数
// 如果为 nil，则使用默认行为（允许所有 Origin，不推荐用于生产环境）
func WithCheckOrigin(checkOrigin func(r *http.Request) bool) Option {
	return func(opts *Options) {
		opts.CheckOrigin = checkOrigin
	}
}

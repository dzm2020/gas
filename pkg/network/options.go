package network

import (
	"gas/pkg/glog"
	"gas/pkg/lib/netutil"
	"net"
	"time"

	"go.uber.org/zap"
)

type Option func(*Options)
type Options struct {
	Handler          IHandler      // 业务回调
	Codec            ICodec        // 协议编解码器
	HeartInterval    time.Duration // 连接超时时间（0表示不检测超时）
	SendChanSize     int           // 发送队列缓冲大小
	ReadBufSize      int           // 读缓冲区大小
	TLSCertFile      string        // TLS 证书文件路径
	TLSKeyFile       string        // TLS 私钥文件路径
	SocketRecvBuffer int           // Socket 接收缓冲区大小（字节）
	SocketSendBuffer int           // Socket 发送缓冲区大小（字节）
	ReusePort        bool          // 是否启用 SO_REUSEPORT（仅 Linux 支持）
	ReuseAddr        bool          // 是否启用 SO_REUSEADDR
	TCPKeepAlive     time.Duration // TCP KeepAlive 时间间隔
	TCPKeepInterval  time.Duration // TCP KeepAlive 探测间隔
	TCPNoDelay       bool          // 是否禁用 TCP Nagle 算法
	LingerOnOff      bool          // 是否启用 SO_LINGER
	LingerSec        int           // SO_LINGER 延迟关闭时间（秒）
	UdpSendChanSize  int
	UdpRcvChanSize   int
}

func loadOptions(options ...Option) *Options {
	opts := &Options{
		SendChanSize:     1024 * 4,
		ReadBufSize:      1024 * 4,
		HeartInterval:    5 * time.Second,
		UdpRcvChanSize:   1024,
		UdpSendChanSize:  1024,
		ReuseAddr:        true,  // 默认启用 SO_REUSEADDR
		ReusePort:        false, // 默认不启用 SO_REUSEPORT（仅 Linux 支持）
		TCPNoDelay:       false, // 默认不禁用 Nagle 算法
		LingerOnOff:      false, // 默认不启用 SO_LINGER
		SocketRecvBuffer: 0,     // 默认使用系统默认值
		SocketSendBuffer: 0,     // 默认使用系统默认值
	}
	for _, option := range options {
		option(opts)
	}
	return opts
}

// WithSendChanSize 设置发送队列缓冲大小
func WithSendChanSize(size int) Option {
	return func(opts *Options) {
		opts.SendChanSize = size
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
		opts.HeartInterval = heartInterval
	}
}

// WithTLS 设置 TLS 证书和私钥文件路径
func WithTLS(certFile, keyFile string) Option {
	return func(opts *Options) {
		opts.TLSCertFile = certFile
		opts.TLSKeyFile = keyFile
	}
}

// WithSocketRecvBuffer 设置 Socket 接收缓冲区大小（字节）
func WithSocketRecvBuffer(size int) Option {
	return func(opts *Options) {
		if size > 0 {
			opts.SocketRecvBuffer = size
		}
	}
}

// WithSocketSendBuffer 设置 Socket 发送缓冲区大小（字节）
func WithSocketSendBuffer(size int) Option {
	return func(opts *Options) {
		if size > 0 {
			opts.SocketSendBuffer = size
		}
	}
}

// WithLinger 设置 SO_LINGER 选项（优雅关闭）
func WithLinger(enable bool, sec int) Option {
	return func(opts *Options) {
		opts.LingerOnOff = enable
		if enable && sec > 0 {
			opts.LingerSec = sec
		}
	}
}

// WithTCPKeepInterval 设置 TCP KeepAlive 探测间隔
func WithTCPKeepInterval(interval time.Duration) Option {
	return func(opts *Options) {
		if interval > 0 {
			opts.TCPKeepInterval = interval
		}
	}
}

// WithTCPNoDelay 设置是否禁用 TCP Nagle 算法
func WithTCPNoDelay(enable bool) Option {
	return func(opts *Options) {
		opts.TCPNoDelay = enable
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

// WithTCPKeepAlive 设置 TCP KeepAlive 时间间隔
func WithTCPKeepAlive(interval time.Duration) Option {
	return func(opts *Options) {
		if interval > 0 {
			opts.TCPKeepAlive = interval
		}
	}
}

func WithUdpSendChanSize(udpSendChanSize int) Option {
	return func(opts *Options) {
		opts.UdpSendChanSize = udpSendChanSize
	}
}

func WithUdpRcvChanSize(udpRcvChanSize int) Option {
	return func(opts *Options) {
		opts.UdpRcvChanSize = udpRcvChanSize
	}
}

func setConOptions(opts *Options, conn net.Conn) {
	address := conn.LocalAddr().String()
	// 设置 SO_REUSEADDR
	if opts.ReuseAddr {
		if err := netutil.SetReuseAddr(conn); err != nil {
			glog.Warn("设置 SO_REUSEADDR 失败", zap.String("localAddr", address), zap.Error(err))
		}
	}

	// 设置 SO_REUSEPORT（仅 Linux 支持）
	if opts.ReusePort {
		if err := netutil.SetReusePort(conn); err != nil {
			glog.Warn("设置 SO_REUSEPORT 失败", zap.String("localAddr", address), zap.Error(err))
		}
	}

	// 设置接收缓冲区
	if opts.SocketRecvBuffer > 0 {
		if err := netutil.SetRcvBuffer(conn, opts.SocketRecvBuffer); err != nil {
			glog.Warn("设置 SO_RCVBUF 失败", zap.String("localAddr", address),
				zap.Int("size", opts.SocketRecvBuffer), zap.Error(err))
		}
	}

	// 设置发送缓冲区
	if opts.SocketSendBuffer > 0 {
		if err := netutil.SetSndBuffer(conn, opts.SocketSendBuffer); err != nil {
			glog.Warn("设置 SO_SNDBUF 失败", zap.String("localAddr", address),
				zap.Int("size", opts.SocketSendBuffer), zap.Error(err))
		}
	}

	// TCP 特定选项（仅对 TCP 连接有效）
	_, isTCP := conn.(*net.TCPConn)
	if !isTCP {
		return // 非 TCP 连接，直接返回
	}

	// 设置 TCP KeepAlive
	if opts.TCPKeepAlive > 0 {
		// 如果设置了 TCPKeepInterval，优先使用它作为探测间隔
		period := opts.TCPKeepAlive
		if opts.TCPKeepInterval > 0 {
			period = opts.TCPKeepInterval
		}
		if err := netutil.SetTCPKeepAlive(conn, true, period); err != nil {
			glog.Warn("设置 TCP KeepAlive 失败", zap.String("localAddr", address),
				zap.Duration("interval", period), zap.Error(err))
		}
	} else if opts.TCPKeepInterval > 0 {
		// 如果只设置了 TCPKeepInterval，也启用 KeepAlive
		if err := netutil.SetTCPKeepAlive(conn, true, opts.TCPKeepInterval); err != nil {
			glog.Warn("设置 TCP KeepAlive 失败", zap.String("localAddr", address),
				zap.Duration("interval", opts.TCPKeepInterval), zap.Error(err))
		}
	}

	// 设置 TCP_NODELAY
	if opts.TCPNoDelay {
		if err := netutil.SetTCPNoDelay(conn, true); err != nil {
			glog.Warn("设置 TCP_NODELAY 失败", zap.String("localAddr", address), zap.Error(err))
		}
	}

	// 设置 SO_LINGER
	if opts.LingerOnOff {
		if err := netutil.SetTCPLinger(conn, true, opts.LingerSec); err != nil {
			glog.Warn("设置 SO_LINGER 失败", zap.String("localAddr", address),
				zap.Int("lingerSec", opts.LingerSec), zap.Error(err))
		}
	}
}

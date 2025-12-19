package network

import (
	"context"
	"fmt"
	"gas/pkg/glog"
	"gas/pkg/lib"
	"net"
	"sync/atomic"

	"go.uber.org/zap"
)

// ------------------------------ TCP服务器 ------------------------------

type TCPServer struct {
	options     *Options
	listener    net.Listener // TCP监听器
	proto, addr string       // 监听地址（如 ":8080"）
	running     atomic.Bool  // 运行状态（原子操作）
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(proto, addr string, option ...Option) *TCPServer {
	return &TCPServer{
		options: loadOptions(option...),
		addr:    addr,
		proto:   proto,
	}
}

func (s *TCPServer) Start() error {
	if !s.running.CompareAndSwap(false, true) {
		return ErrTCPServerAlreadyRunning
	}
	var err error
	if s.listener, err = net.Listen(s.proto, s.addr); err != nil {
		glog.Error("TCP服务器监听失败", zap.String("proto", s.proto),
			zap.String("addr", s.addr), zap.Error(err))
		return err
	}

	glog.Info("TCP服务器启动监听", zap.String("proto", s.proto), zap.String("addr", s.addr))

	lib.Go(func(ctx context.Context) {
		s.acceptLoop(ctx)
	})
	return nil
}

func (s *TCPServer) acceptLoop(ctx context.Context) {
	defer glog.Info("TCP服务器退出监听", zap.String("proto", s.proto), zap.String("addr", s.addr))

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if !s.accept() {
				return
			}
		}
	}
}

func (s *TCPServer) accept() bool {
	if !s.running.Load() {
		return false
	}
	conn, err := s.listener.Accept()
	if err != nil {
		glog.Error("TCP服务器ACCEPT失败", zap.String("proto", s.proto), zap.String("addr", s.addr), zap.Error(err))
		return false
	}
	_ = newTCPConnection(conn, Accept, s.options)
	return true
}

func (s *TCPServer) Addr() string {
	return fmt.Sprintf("%s:%s", s.proto, s.addr)
}

func (s *TCPServer) Stop() error {
	if !s.running.CompareAndSwap(true, false) {
		return ErrTCPServerNotRunning
	}

	if s.listener != nil {
		_ = s.listener.Close()
	}
	glog.Info("TCP服务器关闭", zap.String("proto", s.proto), zap.String("addr", s.addr))
	return nil
}

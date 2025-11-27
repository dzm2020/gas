package network

import (
	"context"
	"errors"
	"fmt"
	"gas/pkg/lib/glog"
	"gas/pkg/lib/workers"
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
		return errors.New("tcp server already running")
	}
	var err error
	if s.listener, err = net.Listen(s.proto, s.addr); err != nil {
		return err
	}
	glog.Info("tcp server listening", zap.String("proto", s.proto), zap.String("addr", s.addr))

	workers.Go(func(ctx context.Context) {
		s.acceptLoop(ctx)
	})
	return nil
}

func (s *TCPServer) acceptLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			s.accept()
		}
	}
}

func (s *TCPServer) accept() {
	if !s.running.Load() {
		return
	}
	conn, err := s.listener.Accept()
	if err != nil {
		glog.Error("tcp server accept failed", zap.String("addr", s.addr), zap.Error(err))
		return
	}
	_ = newTCPConnection(conn, Accept, s.options)
}

func (s *TCPServer) Addr() string {
	return fmt.Sprintf("%s:%s", s.proto, s.addr)
}

func (s *TCPServer) Stop() error {
	if !s.running.CompareAndSwap(true, false) {
		return errors.New("tcp server not running")
	}

	if s.listener != nil {
		_ = s.listener.Close()
	}
	return nil
}

package network

import (
	"errors"
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
	context     *workers.WaitContext
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(proto, addr string, option ...Option) *TCPServer {
	return &TCPServer{
		options: loadOptions(option...),
		addr:    addr,
		proto:   proto,
	}
}

func (s *TCPServer) Start(ctx *workers.WaitContext) error {
	if !s.running.CompareAndSwap(false, true) {
		return errors.New("tcp server already running")
	}
	s.context = workers.WithWaitContext(ctx)
	return s.acceptLoop()
}

func (s *TCPServer) acceptLoop() error {
	defer func() {
		_ = s.Stop()
	}()
	var err error
	if s.listener, err = net.Listen(s.proto, s.addr); err != nil {
		return err
	}
	glog.Info("tcp server listening", zap.String("proto", s.proto), zap.String("addr", s.addr))
	for {
		select {
		case <-s.context.IsFinish():
			return nil
		default:
			s.accept()
		}
	}
}

func (s *TCPServer) accept() {
	conn, err := s.listener.Accept()
	if err != nil {
		glog.Error("tcp server accept failed", zap.String("addr", s.addr), zap.Error(err))
		return
	}
	_ = newTCPConnection(s.context, conn, Accept, s.options)
}

func (s *TCPServer) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *TCPServer) Stop() error {
	if !s.running.CompareAndSwap(true, false) {
		return errors.New("tcp server not running")
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.context.Cancel()
	s.context.Wait()
	glog.Info("tcp server stopped address", zap.String("proto", s.proto), zap.String("addr", s.addr))
	return nil
}

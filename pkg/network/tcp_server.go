package network

import (
	"errors"
	"gas/pkg/utils/glog"
	"net"
	"sync/atomic"

	"go.uber.org/zap"
)

// ------------------------------ TCP服务器 ------------------------------

type TCPServer struct {
	options     *Options
	listener    net.Listener  // TCP监听器
	proto, addr string        // 监听地址（如 ":8080"）
	running     atomic.Bool   // 运行状态（原子操作）
	closeChan   chan struct{} // 关闭信号
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(proto, addr string, option ...Option) *TCPServer {
	return &TCPServer{
		options:   loadOptions(option...),
		addr:      addr,
		proto:     proto,
		closeChan: make(chan struct{}),
	}
}

func (s *TCPServer) Start() error {
	defer func() {
		if r := recover(); r != nil {
			glog.Error("tcp server start panic", zap.Any("panic", r), zap.String("addr", s.addr))
		}
		_ = s.Stop()
	}()

	if !s.running.CompareAndSwap(false, true) {
		return errors.New("tcp server already running")
	}

	var err error
	if s.listener, err = net.Listen(s.proto, s.addr); err != nil {
		return err
	}

	glog.Info("tcp server listening", zap.String("proto", s.proto), zap.String("addr", s.addr))
	for {
		select {
		case <-s.closeChan:
			return nil
		default:
			s.accept()
		}
	}
}

func (s *TCPServer) accept() {
	defer func() {
		if r := recover(); r != nil {
			glog.Error("tcp server accept panic", zap.Any("panic", r), zap.String("addr", s.addr))
		}
	}()
	conn, err := s.listener.Accept()
	if err != nil {
		glog.Error("tcp server accept failed", zap.String("addr", s.addr), zap.Error(err))
		return
	}
	_ = newTCPConnection(conn, Accept, s.options)
}
func (s *TCPServer) Addr() net.Addr {
	return s.listener.Addr()
}
func (s *TCPServer) Stop() error {
	if !s.running.CompareAndSwap(true, false) {
		return errors.New("tcp server not running")
	}
	close(s.closeChan)
	if s.listener != nil {
		_ = s.listener.Close()
	}
	glog.Info("tcp server stopped address", zap.String("proto", s.proto), zap.String("addr", s.addr))
	return nil
}

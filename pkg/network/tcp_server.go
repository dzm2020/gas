package network

import (
	"context"
	"errors"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"gas/pkg/lib/stopper"
	"net"
	"sync"

	"go.uber.org/zap"
)

// ------------------------------ TCP服务器 ------------------------------

type TCPServer struct {
	stopper.Stopper
	options     *Options
	listener    net.Listener // TCP监听器
	proto, addr string       // 监听地址（如 ":8080"）
	protoAddr   string
	waitGroup   sync.WaitGroup
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(protoAddr, proto, addr string, option ...Option) *TCPServer {
	return &TCPServer{
		options:   loadOptions(option...),
		addr:      addr,
		proto:     proto,
		protoAddr: protoAddr,
	}
}

func (s *TCPServer) Addr() string {
	return s.protoAddr
}

func (s *TCPServer) Start() (err error) {
	if s.listener, err = net.Listen(s.proto, s.addr); err != nil {
		return
	}

	grs.Go(func(ctx context.Context) {
		glog.Debug("TCP服务器监听协程启动", zap.String("address", s.Addr()))
		s.acceptLoop()
		glog.Debug("TCP服务器监听协程关闭", zap.String("address", s.Addr()))
	})

	return
}

func (s *TCPServer) acceptLoop() {
	for !s.IsStop() {
		s.accept()
	}
}

func (s *TCPServer) accept() {
	conn, err := s.listener.Accept()
	if err != nil {
		if !errors.Is(err, net.ErrClosed) {
			glog.Error("TCP服务器退出ACCEPT协程", zap.String("address", s.Addr()), zap.Error(err))
		}
		return
	}
	connection := newTCPConnection(conn.(*net.TCPConn), Accept, s.options)
	AddConnection(connection)
	return
}

func (s *TCPServer) Shutdown(ctx context.Context) (err error) {
	if !s.Stop() {
		return
	}
	glog.Debug("TCP服务器关闭", zap.String("address", s.Addr()))

	if s.listener != nil {
		if err = s.listener.Close(); err != nil {
			return
		}
	}
	return
}

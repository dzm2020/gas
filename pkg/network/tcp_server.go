package network

import (
	"context"
	"errors"
	"net"

	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/grs"
	"github.com/dzm2020/gas/pkg/lib/netutil"

	"go.uber.org/zap"
)

// ------------------------------ TCP服务器 ------------------------------

// NewTCPServer 创建TCP服务器
func NewTCPServer(base *baseServer) *TCPServer {
	return &TCPServer{
		baseServer: base,
	}
}

type TCPServer struct {
	*baseServer
	listener net.Listener // TCP监听器
}

func (s *TCPServer) Start() (err error) {
	config := netutil.ListenConfig{
		ReuseAddr: s.options.ReuseAddr,
		ReusePort: s.options.ReusePort,
	}
	if s.listener, err = config.Listen(s.ctx, s.network, s.address); err != nil {
		return
	}

	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		s.acceptLoop()
		s.waitGroup.Done()
	})
	return
}

func (s *TCPServer) acceptLoop() {
	for !s.IsStop() {
		grs.Try(func() {
			s.accept()
		}, nil)
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
	s.newTcpCon(conn)
}

func (s *TCPServer) newTcpCon(conn net.Conn) {
	tcpCon, ok := conn.(*net.TCPConn)
	if !ok {
		glog.Error("连接类型错误，期望 *net.TCPConn", zap.String("address", s.Addr()))
		_ = conn.Close()
		return
	}
	connection := newTCPConnection(s.ctx, tcpCon, Accept, s.options)
	AddConnection(connection)

	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		connection.readLoop()
		s.waitGroup.Done()
		glog.Debug("TCP连接读协程关闭", zap.Int64("connectionId", connection.ID()))
	})

	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		connection.writeLoop()
		s.waitGroup.Done()
		glog.Debug("TCP连接写协程关闭", zap.Int64("connectionId", connection.ID()))
	})

	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		connection.heartLoop(connection)
		s.waitGroup.Done()
	})
}

func (s *TCPServer) Shutdown(ctx context.Context) {
	if !s.Stop() {
		return
	}

	s.baseServer.Shutdown(ctx)
	_ = s.listener.Close()

	glog.Debug("TCP服务器关闭", zap.String("address", s.Addr()))
	return
}

package network

import (
	"context"
	"errors"
	"fmt"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"gas/pkg/lib/stopper"
	"net"
	"sync"

	"github.com/duke-git/lancet/v2/maputil"
	"go.uber.org/zap"
)

// ------------------------------ TCP服务器 ------------------------------

type TCPServer struct {
	stopper.Stopper
	options          *Options
	listener         net.Listener // TCP监听器
	network, address string       // 监听地址（如 ":8080"）
	protoAddress     string
	connDict         *maputil.ConcurrentMap[int64, IConnection]
	waitGroup        sync.WaitGroup
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(ctx context.Context, network, address string, option ...Option) *TCPServer {
	server := &TCPServer{
		options:      loadOptions(option...),
		network:      network,
		address:      address,
		protoAddress: fmt.Sprintf("%s:%s", network, address),
		connDict:     maputil.NewConcurrentMap[int64, IConnection](10),
	}
	server.ctx, server.cancel = context.WithCancel(ctx)
	return server
}

func (s *TCPServer) Addr() string {
	return s.protoAddress
}

func (s *TCPServer) Start() (err error) {
	if s.listener, err = net.Listen(s.network, s.address); err != nil {
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
	tcpCon := conn.(*net.TCPConn)
	connection := newTCPConnection(s.ctx, tcpCon, Accept, s.options)
	AddConnection(connection)
	s.connDict.Set(connection.ID(), connection)

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

	return
}

func (s *TCPServer) Shutdown(ctx context.Context) {
	if !s.Stop() {
		return
	}
	glog.Debug("TCP服务器开始关闭", zap.String("address", s.Addr()))

	if s.listener != nil {
		_ = s.listener.Close()
	}

	s.cancel()

	grs.GroupWaitWithContext(ctx, &s.waitGroup)

	glog.Debug("TCP服务器已关闭", zap.String("address", s.Addr()))
	return
}

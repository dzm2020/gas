package network

import (
	"context"
	"fmt"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"io"
	"net"
	"sync"

	"go.uber.org/zap"
)

// ------------------------------ TCP服务器 ------------------------------

type TCPServer struct {
	options     *Options
	listener    net.Listener // TCP监听器
	proto, addr string       // 监听地址（如 ":8080"）
	address     string
	once        sync.Once
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(proto, addr string, option ...Option) *TCPServer {
	return &TCPServer{
		options: loadOptions(option...),
		addr:    addr,
		proto:   proto,
		address: fmt.Sprintf("%s:%s", proto, addr),
	}
}

func (s *TCPServer) Addr() string {
	return s.address
}

func (s *TCPServer) Start() error {
	var err error
	if s.listener, err = net.Listen(s.proto, s.addr); err != nil {
		glog.Error("TCP服务器监听失败", zap.String("address", s.Addr()), zap.Error(err))
		return err
	}

	grs.Go(func(ctx context.Context) {
		s.acceptLoop()
	})

	glog.Info("TCP服务器启动监听", zap.String("address", s.Addr()))
	return nil
}

func (s *TCPServer) acceptLoop() {
	for {
		if err := s.accept(); err != nil {
			return
		}
	}
}

func (s *TCPServer) accept() error {
	defer grs.Recover(func(err any) {
		glog.Info("TCP服务器", zap.String("address", s.Addr()), zap.Any("err", err))
	})
	if s.listener == nil {
		return ErrListenerIsNil
	}
	conn, err := s.listener.Accept()
	if err != nil {
		if err != io.EOF {
			glog.Info("TCP服务器退出ACCEPT协程", zap.String("address", s.Addr()), zap.Error(err))
		}
		return err
	}
	connection := newTCPConnection(conn, Accept, s.options)
	AddConnection(connection)
	return nil
}

func (s *TCPServer) Shutdown() (err error) {
	s.once.Do(func() {
		glog.Info("TCP服务器关闭", zap.String("address", s.Addr()))
		if s.listener != nil {
			if err = s.listener.Close(); err != nil {
				return
			}
		}
	})
	return
}

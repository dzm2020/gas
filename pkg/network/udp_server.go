package network

import (
	"context"
	"fmt"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"io"
	"net"
	"sync"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/zap"
)

// ------------------------------ UDP服务器 ------------------------------

type UDPServer struct {
	options      *Options
	conn         *net.UDPConn // UDP监听连接
	proto, addr  string       // 监听地址
	connections  map[string]*UDPConnection
	rwMutex      sync.RWMutex // 保护connections并发
	protoAddress string
	once         sync.Once
}

// NewUDPServer 创建UDP服务器
func NewUDPServer(proto, addr string, option ...Option) *UDPServer {
	opts := loadOptions(option...)
	return &UDPServer{
		options:      opts,
		addr:         addr,
		proto:        proto,
		connections:  make(map[string]*UDPConnection),
		protoAddress: fmt.Sprintf("%s:%s", proto, addr),
	}
}

func (s *UDPServer) Addr() string {
	return s.protoAddress
}

func (s *UDPServer) removeConnection(connKey string) {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	delete(s.connections, connKey)
}

func (s *UDPServer) getConnection(connKey string) (*UDPConnection, bool) {
	s.rwMutex.RLock()
	defer s.rwMutex.RUnlock()
	conn, ok := s.connections[connKey]
	return conn, ok
}
func (s *UDPServer) addConnection(connKey string, remoteAddr *net.UDPAddr) *UDPConnection {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	// 双重检查，避免并发创建
	udpConn, exists := s.connections[connKey]
	if !exists {
		udpConn = newUDPConnection(s.conn, Accept, remoteAddr, s)
		AddConnection(udpConn)
		s.connections[connKey] = udpConn
	}
	return udpConn
}

func (s *UDPServer) Listen() error {
	if err := s.listen(); err != nil {
		glog.Error("UDP服务器监听错误", zap.String("address", s.Addr()), zap.Error(err))
		return err
	}
	grs.Go(func(ctx context.Context) {
		s.readLoop()
	})
	glog.Info("UDP服务器监听", zap.String("address", s.Addr()))
	return nil

}

func (s *UDPServer) listen() error {
	udpAddr, err := net.ResolveUDPAddr(s.proto, s.addr)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP(s.proto, udpAddr)
	return err
}

func (s *UDPServer) readLoop() {
	var err error
	defer glog.Info("UDP服务器退出READ协程", zap.String("address", s.Addr()), zap.Error(err))
	for {
		if err = s.read(); err != nil {
			return
		}
	}
}

func (s *UDPServer) read() error {
	defer grs.Recover(func(err any) {
		glog.Panic("UDP服务器", zap.String("address", s.Addr()), zap.Any("err", err))
	})
	buf := make([]byte, s.options.readBufSize)
	n, remoteAddr, w := s.conn.ReadFromUDP(buf)
	if w != nil {
		return w
	}
	if n == 0 {
		return io.EOF
	}
	remoteAddrCopy := convertor.DeepClone(remoteAddr)
	s.handlePacket(remoteAddrCopy, buf[:n])
	return nil
}

// handlePacket 处理UDP数据包
func (s *UDPServer) handlePacket(remoteAddr *net.UDPAddr, buf []byte) {
	connKey := remoteAddr.String()
	udpConn, exists := s.getConnection(connKey)
	if !exists {
		udpConn = s.addConnection(connKey, remoteAddr)
	}
	udpConn.input(buf)
}

func (s *UDPServer) Shutdown() (err error) {
	s.once.Do(func() {
		if s.conn != nil {
			if err = s.conn.Close(); err != nil {
				return
			}
		}
	})
	return nil
}

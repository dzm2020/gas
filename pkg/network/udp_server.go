package network

import (
	"errors"
	"gas/pkg/utils/glog"
	"net"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// ------------------------------ UDP服务器 ------------------------------

type UDPServer struct {
	options     *Options
	conn        *net.UDPConn  // UDP监听连接
	proto, addr string        // 监听地址
	running     atomic.Bool   // 运行状态
	closeChan   chan struct{} // 关闭信号
	connections map[string]*UDPConnection
	rwMutex     sync.RWMutex // 保护connections并发
}

// NewUDPServer 创建UDP服务器
func NewUDPServer(proto, addr string, option ...Option) *UDPServer {
	opts := loadOptions(option...)
	return &UDPServer{
		options:     opts,
		addr:        addr,
		proto:       proto,
		closeChan:   make(chan struct{}),
		connections: make(map[string]*UDPConnection),
	}
}

func (s *UDPServer) Start() error {
	defer func() {
		if r := recover(); r != nil {
			glog.Error("udp server start panic", zap.Any("panic", r), zap.String("addr", s.addr))
		}
		_ = s.Stop()
	}()

	if !s.running.CompareAndSwap(false, true) {
		return errors.New("udp server already running")
	}

	if err := s.listen(); err != nil {
		glog.Error("udp server listen error", zap.String("proto", s.proto),
			zap.String("addr", s.addr), zap.Error(err))
		return err
	}

	glog.Info("udp server listen", zap.String("proto", s.proto),
		zap.String("addr", s.addr))

	readBuf := make([]byte, s.options.readBufSize)
	for {
		select {
		case <-s.closeChan:
			return nil
		default:
			n, remoteAddr, err := s.conn.ReadFromUDP(readBuf)
			if err != nil {
				return err
			}
			if n == 0 {
				continue
			}

			s.handlePacket(remoteAddr, readBuf, n)
		}
	}
}

func (s *UDPServer) listen() error {
	udpAddr, err := net.ResolveUDPAddr(s.proto, s.addr)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP(s.proto, udpAddr)
	return err
}

// copyUDPAddr 复制 UDPAddr（避免并发问题）
func copyUDPAddr(addr *net.UDPAddr) *net.UDPAddr {
	ip := make(net.IP, len(addr.IP))
	copy(ip, addr.IP)
	return &net.UDPAddr{IP: ip, Port: addr.Port, Zone: addr.Zone}
}

// handlePacket 处理UDP数据包（在协程池中并发执行）
func (s *UDPServer) handlePacket(remoteAddr *net.UDPAddr, readBuf []byte, n int) {
	data := make([]byte, n)
	copy(data, readBuf[:n])
	remoteAddrCopy := copyUDPAddr(remoteAddr)

	connKey := remoteAddrCopy.String()

	s.rwMutex.RLock()
	udpConn, exists := s.connections[connKey]
	s.rwMutex.RUnlock()

	if !exists {
		s.rwMutex.Lock()
		// 双重检查，避免并发创建
		if udpConn, exists = s.connections[connKey]; !exists {
			udpConn = newUDPConnection(s.conn, Accept, remoteAddrCopy, s)
			s.connections[connKey] = udpConn
		}
		s.rwMutex.Unlock()
	}

	udpConn.input(data)
}

func (s *UDPServer) removeConnection(connKey string) {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	delete(s.connections, connKey)
}

func (s *UDPServer) Addr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *UDPServer) Stop() error {
	if !s.running.CompareAndSwap(true, false) {
		return errors.New("udp server not running")
	}
	close(s.closeChan)
	if s.conn != nil {
		_ = s.conn.Close()
	}
	return nil
}

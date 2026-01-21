package network

import (
	"context"
	"errors"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"gas/pkg/lib/netutil"
	"io"
	"net"
	"sync"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/zap"
)

type udpPacket struct {
	data       []byte
	remoteAddr *net.UDPAddr
}

// ------------------------------ UDP服务器 ------------------------------

// NewUDPServer 创建UDP服务器
func NewUDPServer(base *baseServer) *UDPServer {
	server := &UDPServer{
		baseServer:  base,
		connections: make(map[string]*UDPConnection),
		sendChan:    make(chan *udpPacket, base.options.UdpSendChanSize),
	}
	return server
}

type UDPServer struct {
	*baseServer
	conn        *net.UDPConn // UDP监听连接
	connections map[string]*UDPConnection
	rwMutex     sync.RWMutex // 保护connections并发
	sendChan    chan *udpPacket
}

func (s *UDPServer) getSendChan() chan<- *udpPacket {
	return s.sendChan
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
		udpConn = newUDPConnection(s.ctx, s.conn, Accept, remoteAddr, s)

		s.waitGroup.Add(1)
		grs.Go(func(ctx context.Context) {
			udpConn.readLoop()
			s.waitGroup.Done()
		})

		AddConnection(udpConn)
		s.connections[connKey] = udpConn
	}
	return udpConn
}

func (s *UDPServer) Start() error {
	if err := s.listen(); err != nil {
		glog.Error("UDP服务器监听错误", zap.String("address", s.Addr()), zap.Error(err))
		return err
	}
	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		s.readLoop()
		s.waitGroup.Done()
	})
	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		s.writeLoop()
		s.waitGroup.Done()
	})
	glog.Info("UDP服务器监听", zap.String("address", s.Addr()))
	return nil

}

func (s *UDPServer) listen() (err error) {
	config := netutil.ListenConfig{
		ReuseAddr: s.options.ReuseAddr,
		ReusePort: s.options.ReusePort,
	}
	var ln net.PacketConn
	ln, err = config.ListenPacket(s.ctx, s.network, s.address)

	s.conn = ln.(*net.UDPConn)

	setConOptions(s.options, s.conn)
	return err
}

func (s *UDPServer) readLoop() {
	for !s.IsStop() {
		grs.Try(func() {
			s.read()
		}, nil)
	}
}

func (s *UDPServer) read() {
	buf := make([]byte, s.options.ReadBufSize)
	n, remoteAddr, err := s.conn.ReadFromUDP(buf)
	if err != nil {
		if err != io.EOF {
			glog.Error("UDP服务器读取数据包异常", zap.String("address", s.Addr()), zap.Any("err", err))
		}
		return
	}
	if n == 0 {
		return
	}
	remoteAddrCopy := convertor.DeepClone(remoteAddr)
	s.handlePacket(remoteAddrCopy, buf[:n])
	return
}

// handlePacket 处理UDP数据包
func (s *UDPServer) handlePacket(remoteAddr *net.UDPAddr, buf []byte) {
	connKey := remoteAddr.String()
	udpConn, exists := s.getConnection(connKey)
	if !exists {
		udpConn = s.addConnection(connKey, remoteAddr)
	}
	udpConn.writeRcvChan(buf)
}

func (s *UDPServer) writeLoop() {
	for !s.IsStop() {
		grs.Try(func() {
			s.write()
		}, nil)
	}
}

func (s *UDPServer) write() {
	select {
	case <-s.ctx.Done():
		return
	case packet, _ := <-s.sendChan:
		_, err := s.conn.WriteToUDP(packet.data, packet.remoteAddr)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				glog.Error("UDP服务器写入失败", zap.String("address", s.Addr()), zap.Error(err))
			}
			return
		}
	}
}

func (s *UDPServer) Shutdown(ctx context.Context) {
	if !s.Stop() {
		return
	}

	glog.Debug("UDP服务器开始关闭", zap.String("address", s.Addr()))

	s.cancel()

	if s.conn != nil {
		_ = s.conn.Close()
	}

	grs.GroupWaitWithContext(ctx, &s.waitGroup)

	glog.Debug("UDP服务器已关闭", zap.String("address", s.Addr()))
	return
}

package network

import (
	"context"
	"errors"
	"fmt"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
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

type UDPServer struct {
	options          *Options
	conn             *net.UDPConn // UDP监听连接
	network, address string       // 监听地址
	connections      map[string]*UDPConnection
	rwMutex          sync.RWMutex // 保护connections并发
	protoAddress     string
	once             sync.Once
	sendChan         chan *udpPacket
}

// NewUDPServer 创建UDP服务器
func NewUDPServer(network, address string, option ...Option) *UDPServer {
	opts := loadOptions(option...)
	return &UDPServer{
		options:      opts,
		network:      network,
		address:      address,
		connections:  make(map[string]*UDPConnection),
		protoAddress: fmt.Sprintf("%s:%s", network, address),
		sendChan:     make(chan *udpPacket, 1024),
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

func (s *UDPServer) Start() error {
	if err := s.listen(); err != nil {
		glog.Error("UDP服务器监听错误", zap.String("address", s.Addr()), zap.Error(err))
		return err
	}
	grs.Go(func(ctx context.Context) {
		s.readLoop()
	})
	grs.Go(func(ctx context.Context) {
		s.writeLoop()
	})
	glog.Info("UDP服务器监听", zap.String("address", s.Addr()))
	return nil

}

func (s *UDPServer) listen() error {
	udpAddr, err := net.ResolveUDPAddr(s.network, s.address)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP(s.network, udpAddr)
	return err
}

func (s *UDPServer) readLoop() {
	for {
		if err := s.read(); err != nil {
			return
		}
	}
}

func (s *UDPServer) read() error {
	defer grs.Recover(func(err any) {
		glog.Error("UDP服务器读取数据包异常", zap.String("address", s.Addr()), zap.Any("err", err))
	})

	buf := make([]byte, s.options.readBufSize)
	n, remoteAddr, err := s.conn.ReadFromUDP(buf)
	if err != nil {
		if err != io.EOF {
			glog.Error("UDP服务器读取数据包异常", zap.String("address", s.Addr()), zap.Any("err", err))
		}
		return err
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

func (s *UDPServer) writeLoop() {
	defer grs.Recover(func(err any) {
		glog.Error("UDP服务器写入异常", zap.String("address", s.Addr()), zap.Any("err", err))
	})
	for {
		select {
		case packet, ok := <-s.sendChan:
			if !ok {
				return // 通道已关闭
			}
			_, err := s.conn.WriteToUDP(packet.data, packet.remoteAddr)
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					glog.Error("UDP服务器写入失败", zap.String("address", s.Addr()), zap.Error(err))
				}
				return
			}
		}
	}
}

func (s *UDPServer) send(data []byte, remoteAddr *net.UDPAddr) error {
	if data == nil || remoteAddr == nil {
		return nil
	}
	select {
	case s.sendChan <- &udpPacket{data: data, remoteAddr: remoteAddr}:
	default:
		return errors.New("channel is full")
	}
	return nil
}

func (s *UDPServer) Shutdown(ctx context.Context) {
	s.once.Do(func() {
		if s.sendChan != nil {
			close(s.sendChan)
		}
		if s.conn != nil {
			_ = s.conn.Close()
		}
	})
	return
}

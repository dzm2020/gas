package network

import (
	"context"
	"errors"
	"net"

	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/grs"
	"github.com/dzm2020/gas/pkg/lib/netutil"

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
		baseServer: base,
		sendChan:   make(chan *udpPacket, base.options.UdpSendChanSize),
		tmpBuf:     make([]byte, base.options.ReadBufSize),
	}
	return server
}

type UDPServer struct {
	*baseServer
	tmpBuf   []byte
	conn     *net.UDPConn // UDP监听连接
	sendChan chan *udpPacket
}

func (s *UDPServer) getSendChan() chan<- *udpPacket {
	return s.sendChan
}

func (s *UDPServer) Start() error {
	if err := s.listen(); err != nil {
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
	if err != nil {
		return err
	}

	udpConn, ok := ln.(*net.UDPConn)
	if !ok {
		return errors.New("listener is not *net.UDPConn")
	}

	s.conn = udpConn
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
	n, remoteAddr, err := s.conn.ReadFromUDP(s.tmpBuf)
	if err != nil {
		if !errors.Is(err, net.ErrClosed) {
			glog.Error("UDP服务器读取数据包异常", zap.String("address", s.Addr()), zap.Any("err", err))
		}
		return
	}
	if n == 0 {
		return
	}
	remoteAddrCopy := convertor.DeepClone(remoteAddr)
	s.handlePacket(remoteAddrCopy, s.tmpBuf[:n])
	return
}

// handlePacket 处理UDP数据包
func (s *UDPServer) handlePacket(remoteAddr *net.UDPAddr, buf []byte) {
	connKey := remoteAddr.String()
	udpConn, exists := GetUDPConnection(connKey)
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

func (s *UDPServer) addConnection(connKey string, remoteAddr *net.UDPAddr) *UDPConnection {
	// 双重检查，避免并发创建
	udpConn, exists := GetUDPConnection(connKey)
	if exists {
		return udpConn
	}

	udpConn = newUDPConnection(s.ctx, s.conn, Accept, remoteAddr, s)

	// 双重检查：如果添加时已存在，使用已存在的连接
	existingConn, added := AddUDPConnection(connKey, udpConn)
	if !added {
		return existingConn
	}

	AddConnection(udpConn)

	// 只有成功添加后才启动 goroutine
	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		udpConn.readLoop()
		s.waitGroup.Done()
	})

	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		udpConn.heartLoop(udpConn)
		s.waitGroup.Done()
	})

	return udpConn
}

func (s *UDPServer) Shutdown(ctx context.Context) {
	if !s.Stop() {
		return
	}

	s.baseServer.Shutdown(ctx)
	_ = s.conn.Close()

	glog.Debug("UDP服务器已关闭", zap.String("address", s.Addr()))
	return
}

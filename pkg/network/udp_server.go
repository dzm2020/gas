package network

import (
	"context"
	"errors"
	"net"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/grs"
	"github.com/dzm2020/gas/pkg/lib/netutil"

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
		sendChan:   make(chan *udpPacket, base.options.SendChanSize),
	}
	return server
}

type UDPServer struct {
	*baseServer
	conn     *net.UDPConn // UDP监听连接
	sendChan chan *udpPacket
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

	udpConn, _ := ln.(*net.UDPConn)
	s.conn = udpConn
	return err
}

func (s *UDPServer) readLoop() {
	for !s.IsStop() {
		buf := make([]byte, s.options.ReadBufSize)
		n, remoteAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				glog.Error("UDP服务器读取数据包异常", zap.String("address", s.Addr()), zap.Any("err", err))
			}
			continue
		}
		if n == 0 {
			continue
		}

		remoteAddrCopy := convertor.DeepClone(remoteAddr)

		connKey := remoteAddrCopy.String()
		udpConn, exists := GetUDPConnection(connKey)
		if !exists {
			udpConn = s.addConnection(connKey, remoteAddrCopy)
		}
		udpConn.writeRcvChan(buf[:n])
	}
}

func (s *UDPServer) writeLoop() {
	for !s.IsStop() {
		select {
		case <-s.ctx.Done():
			continue
		case packet, _ := <-s.sendChan:
			_, err := s.conn.WriteToUDP(packet.data, packet.remoteAddr)
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					glog.Error("UDP服务器写入失败", zap.String("address", s.Addr()), zap.Error(err))
				}
				continue
			}
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

	_ = s.conn.Close()
	s.baseServer.Shutdown(ctx)

	glog.Debug("UDP服务器已关闭", zap.String("address", s.Addr()))
	return
}

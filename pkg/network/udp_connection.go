package network

import (
	"context"
	"errors"
	"net"

	"github.com/dzm2020/gas/pkg/glog"
	"go.uber.org/zap"
)

// ------------------------------ UDP虚拟连接 ------------------------------

type UDPConnection struct {
	*baseConn  // 嵌入基类
	remoteAddr *net.UDPAddr
	server     *UDPServer   // 所属服务器
	conn       *net.UDPConn // 底层UDP连接（全局共享）
	rcvChan    chan []byte
}

func newUDPConnection(ctx context.Context, conn *net.UDPConn, typ ConnType, remoteAddr *net.UDPAddr, server *UDPServer) *UDPConnection {
	base := newBaseConn(ctx, "udp", typ, conn, server.options)
	base.remoteAddr = remoteAddr

	rcvChanSize := server.options.UdpRcvChanSize
	udpConn := &UDPConnection{
		baseConn:   base,
		remoteAddr: remoteAddr,
		conn:       conn,
		server:     server,
		rcvChan:    make(chan []byte, rcvChanSize),
	}
	return udpConn
}

func (c *UDPConnection) Send(msg interface{}) error {
	if c.IsStop() {
		return ErrConnectionClosed
	}
	data, err := c.encode(msg)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	//  todo 这里是不是处理下 不需要remoteAddr
	ch := c.server.getSendChan()
	select {
	case ch <- &udpPacket{data: data, remoteAddr: c.remoteAddr}:
	default:
		return errors.New("channel is full")
	}
	return nil
}

func (c *UDPConnection) readLoop() {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.onConnect(c); err != nil {
		return
	}

	for !c.IsStop() {
		select {
		case <-c.ctx.Done():
			return
		case data := <-c.rcvChan:
			_, err = c.process(c, data)
			if err != nil {
				return
			}
		}
	}
}

func (c *UDPConnection) Write(p []byte) (n int, err error) {
	_, err = c.conn.WriteTo(p, c.remoteAddr)
	return len(p), err
}

func (c *UDPConnection) writeRcvChan(data []byte) {
	select {
	case c.rcvChan <- data:
	default:
		glog.Error("UDP读取chan已满", zap.Int64("connectionId", c.ID()))
	}
}

func (c *UDPConnection) Close(err error) (w error) {
	if !c.Stop() {
		return ErrConnectionClosed
	}

	RemoveUDPConnection(c.RemoteAddr())
	c.baseConn.Close(c, err)

	glog.Info("UDP连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))
	return
}

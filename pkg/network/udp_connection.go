package network

import (
	"context"
	"errors"
	"gas/pkg/glog"
	"net"

	"go.uber.org/zap"
)

// ------------------------------ UDP虚拟连接 ------------------------------

type UDPConnection struct {
	*baseConnection // 嵌入基类
	remoteAddr      *net.UDPAddr
	server          *UDPServer   // 所属服务器
	conn            *net.UDPConn // 底层UDP连接（全局共享）
	rcvChan         chan []byte
}

func newUDPConnection(ctx context.Context, conn *net.UDPConn, typ ConnectionType, remoteAddr *net.UDPAddr, server *UDPServer) *UDPConnection {
	base := initBaseConnection(ctx, typ, conn.LocalAddr(), remoteAddr, server.options)
	rcvChanSize := server.options.UdpRcvChanSize
	udpConn := &UDPConnection{
		baseConnection: base,
		remoteAddr:     remoteAddr,
		conn:           conn,
		server:         server,
		rcvChan:        make(chan []byte, rcvChanSize),
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
		case <-c.getTimeoutChan():
			if err = c.checkTimeout(); err != nil {
				return
			}
		case data := <-c.rcvChan:
			_, err = c.baseConnection.process(c, data)
			if err != nil {
				return
			}
		}
	}
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

	glog.Info("UDP连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))

	if c.server != nil && c.remoteAddr != nil {
		c.server.removeConnection(c.remoteAddr.String())
	}
	c.baseConnection.Close(c, err)
	return
}

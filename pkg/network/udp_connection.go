package network

import (
	"context"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"net"

	"go.uber.org/zap"
)

// ------------------------------ UDP虚拟连接 ------------------------------

type UDPConnection struct {
	*baseConnection // 嵌入基类
	remoteAddr      *net.UDPAddr
	server          *UDPServer   // 所属服务器
	conn            *net.UDPConn // 底层UDP连接（全局共享）
	readChan        chan []byte
}

func newUDPConnection(conn *net.UDPConn, typ ConnectionType, remoteAddr *net.UDPAddr, server *UDPServer) *UDPConnection {
	base := initBaseConnection(typ, conn.LocalAddr(), remoteAddr, server.options)
	udpConn := &UDPConnection{
		baseConnection: base,
		remoteAddr:     remoteAddr,
		conn:           conn,
		server:         server,
		readChan:       make(chan []byte, 100),
	}

	grs.Go(func(ctx context.Context) {
		udpConn.writeLoop()
	})

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
	return c.server.send(data, c.remoteAddr)
}

func (c *UDPConnection) writeLoop() {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.onConnect(c); err != nil {
		return
	}

	for {
		select {
		case <-c.getTimeoutChan():
			if err = c.checkTimeout(); err != nil {
				return
			}
		case data := <-c.readChan:
			_, err = c.baseConnection.process(c, data)
			if err != nil {
				return
			}
		}
	}
}

func (c *UDPConnection) input(data []byte) {
	select {
	case c.readChan <- data:
	default:
		glog.Error("UDP读取chan已满", zap.Int64("connectionId", c.ID()))
	}
}

func (c *UDPConnection) Close(err error) (w error) {
	if !c.Stop() {
		return ErrConnectionClosed
	}

	glog.Info("UDP连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))

	if c.readChan != nil {
		close(c.readChan)
		c.readChan = nil
	}

	if c.server != nil && c.remoteAddr != nil {
		c.server.removeConnection(c.remoteAddr.String())
	}

	c.baseConnection.Close(c, err)

	return
}

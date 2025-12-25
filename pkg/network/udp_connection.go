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
	*baseConnection              // 嵌入基类
	server          *UDPServer   // 所属服务器
	conn            *net.UDPConn // 底层UDP连接（全局共享）
	readChan        chan []byte
	remoteAddr      *net.UDPAddr // 远程地址（虚拟连接的核心标识）
}

func newUDPConnection(conn *net.UDPConn, typ ConnectionType, remoteAddr *net.UDPAddr, server *UDPServer) *UDPConnection {
	udpConn := &UDPConnection{
		baseConnection: initBaseConnection(typ, server.options),
		conn:           conn,
		remoteAddr:     remoteAddr,
		server:         server,
		readChan:       make(chan []byte, 100),
	}

	glog.Info("创建UDP连接", zap.Int64("connectionId", udpConn.ID()),
		zap.String("localAddr", udpConn.LocalAddr().String()),
		zap.String("remoteAddr", udpConn.RemoteAddr().String()))

	grs.Go(func(ctx context.Context) {
		udpConn.writeLoop(ctx)
	})
	return udpConn
}

func (c *UDPConnection) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *UDPConnection) RemoteAddr() net.Addr { return c.remoteAddr }

func (c *UDPConnection) Send(msg interface{}) error {
	if err := c.checkClosed(); err != nil {
		return err
	}
	data, err := c.encode(msg)
	if err != nil {
		glog.Error("UDP消息encode失败", zap.Int64("connectionId", c.ID()), zap.Error(err))
		return err
	}
	if _, err = c.conn.WriteToUDP(data, c.remoteAddr); err != nil {
		glog.Error("UDP发送消息", zap.Int64("connectionId", c.ID()), zap.Error(err))
		return err
	}
	return nil
}

func (c *UDPConnection) writeLoop(ctx context.Context) {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.onConnect(c); err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closeChan:
			return
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
	if !c.closeBase() {
		return ErrConnectionClosed
	}

	glog.Info("UDP连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))

	if c.server != nil && c.remoteAddr != nil {
		c.server.removeConnection(c.remoteAddr.String())
	}

	if c.baseConnection != nil {
		if w = c.baseConnection.Close(c, err); w != nil {
			return
		}
	}

	return
}

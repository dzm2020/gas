package network

import (
	"context"
	"errors"
	"fmt"
	"gas/pkg/glog"
	"gas/pkg/lib"
	"net"

	"go.uber.org/zap"
)

// ------------------------------ UDP虚拟连接 ------------------------------

type UDPConnection struct {
	*baseConnection              // 嵌入基类
	server          *UDPServer   // 所属服务器
	conn            *net.UDPConn // 底层UDP连接（全局共享）
	writeChan       chan []byte
	remoteAddr      *net.UDPAddr // 远程地址（虚拟连接的核心标识）
}

func newUDPConnection(conn *net.UDPConn, typ ConnectionType, remoteAddr *net.UDPAddr, server *UDPServer) *UDPConnection {
	udpConn := &UDPConnection{
		baseConnection: initBaseConnection(typ, server.options),
		conn:           conn,
		remoteAddr:     remoteAddr,
		server:         server,
		writeChan:      make(chan []byte, 100),
	}

	glog.Infof("udp connection open %v local:%v remote:%v typ:%v", udpConn.ID(), udpConn.LocalAddr(), udpConn.RemoteAddr(), udpConn.Type())

	AddConnection(udpConn)
	lib.Go(func(ctx context.Context) {
		udpConn.writeLoop(ctx)
	})
	return udpConn
}

func (c *UDPConnection) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *UDPConnection) RemoteAddr() net.Addr { return c.remoteAddr }

func (c *UDPConnection) Send(msg interface{}) error {
	if err := c.checkClosed("udp"); err != nil {
		return err
	}
	data, err := c.codec.Encode(msg)
	if err != nil {
		glog.Error("udp encode data error", zap.Int64("conn_id", c.ID()), zap.Error(err), zap.Any("data", msg))
		return err
	}
	if _, err = c.conn.WriteToUDP(data, c.remoteAddr); err != nil {
		glog.Error("udp write data error", zap.Int64("conn_id", c.ID()), zap.Error(err), zap.Any("data", msg))
		return err
	}
	return nil
}

func (c *UDPConnection) writeLoop(ctx context.Context) {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.handler.OnConnect(c); err != nil {
		glog.Error("udp connection OnConnect error", zap.Int64("conn_id", c.ID()), zap.Error(err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closeChan:
			return
		case <-c.timeoutTicker.C:
			if c.isTimeout() {
				err = fmt.Errorf("keepAlive")
				return
			}
		case data := <-c.writeChan:
			_, err = c.baseConnection.process(c, data)
			if err != nil {
				return
			}
		}
	}
}

func (c *UDPConnection) input(data []byte) {
	select {
	case c.writeChan <- data:
	default:
		glog.Warn("udp connection read channel full, closing connection", zap.Int64("conn_id", c.ID()))
		_ = c.Close(errors.New("read channel full"))
	}
}

func (c *UDPConnection) Close(err error) error {
	if !c.closeBase() {
		return errors.New("udp connection already closed")
	}

	lib.Go(func(context.Context) {
		c.server.removeConnection(c.remoteAddr.String())
		_ = c.baseConnection.Close(c, err)
	})
	glog.Info("udp connection close", zap.Int64("conn_id", c.ID()), zap.Error(err))
	return nil
}

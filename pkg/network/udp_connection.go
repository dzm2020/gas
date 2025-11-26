package network

import (
	"errors"
	"fmt"
	"gas/pkg/utils/glog"
	"gas/pkg/utils/workers"
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

	glog.Infof("udp connection open %v local:%v remote:%v typ:%v", udpConn.ID(), udpConn.LocalAddr(), udpConn.RemoteAddr(), udpConn.Type())

	AddConnection(udpConn)
	workers.Submit(func() {
		udpConn.readLoop()
	}, func(err interface{}) {
		glog.Error("udp connection readLoop panic", zap.Any("panic", err), zap.Int64("conn_id", udpConn.ID()))
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

func (c *UDPConnection) readLoop() {
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
		case <-c.closeChanSignal():
			return
		case <-c.timeoutTicker.C:
			if c.isTimeout() {
				err = fmt.Errorf("timeout")
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
		glog.Warn("udp connection read channel full, closing connection", zap.Int64("conn_id", c.ID()))
		_ = c.Close(errors.New("read channel full"))
	}
}

func (c *UDPConnection) Close(err error) error {
	if !c.closeBase() {
		return errors.New("udp connection already closed")
	}
	workers.Submit(func() {
		c.server.removeConnection(c.remoteAddr.String())
		_ = c.baseConnection.Close(c, err)
	}, func(err interface{}) {
		glog.Error("udp connection close panic", zap.Any("panic", err), zap.Int64("conn_id", c.ID()))
	})
	glog.Info("udp connection close", zap.Int64("conn_id", c.ID()), zap.Error(err))
	return nil
}

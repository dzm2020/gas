package network

import (
	"context"
	"gas/pkg/glog"
	"gas/pkg/lib/buffer"
	"gas/pkg/lib/grs"
	"io"
	"net"

	"go.uber.org/zap"
)

type TCPConnection struct {
	*baseConnection // 嵌入基类
	conn            net.Conn
	server          *TCPServer // 所属服务器
	tmpBuf          []byte
	buffer          buffer.IBuffer
}

func newTCPConnection(conn net.Conn, typ ConnectionType, options *Options) *TCPConnection {
	tcpConn := &TCPConnection{
		baseConnection: initBaseConnection(typ, options),
		tmpBuf:         make([]byte, options.readBufSize),
		buffer:         buffer.New(options.readBufSize),
		conn:           conn,
	}

	grs.Go(func(ctx context.Context) {
		tcpConn.readLoop()
	})

	grs.Go(func(ctx context.Context) {
		tcpConn.writeLoop()
	})

	glog.Info("创建TCP连接", zap.Int64("connectionId", tcpConn.ID()),
		zap.String("localAddr", tcpConn.LocalAddr().String()),
		zap.String("remoteAddr", tcpConn.RemoteAddr().String()))
	return tcpConn
}

func (c *TCPConnection) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *TCPConnection) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

func (c *TCPConnection) readLoop() {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.onConnect(c); err != nil {
		return
	}
	for {
		if err = c.read(); err != nil {
			return
		}
	}
}

func (c *TCPConnection) read() error {
	n, readErr := c.conn.Read(c.tmpBuf)

	if readErr != nil {
		return readErr
	}

	if n == 0 {
		return io.EOF
	}

	if _, readErr = c.buffer.Write(c.tmpBuf[:n]); readErr != nil {
		return readErr
	}

	// 循环解码（处理粘包，可能一次读取多个消息）
	for c.buffer.Len() > 0 {
		pn, err := c.process(c, c.buffer.Bytes())
		if pn == 0 {
			break // 数据不完整，等待下一次读取
		}
		err = c.buffer.Skip(pn)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *TCPConnection) writeLoop() {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	var msg interface{}
	var ok bool
	for {
		select {
		case msg, ok = <-c.sendChan:
			if !ok {
				return // 通道已关闭
			}
			if err = c.batchWriteMsg(msg); err != nil {
				return
			}
		case <-c.getTimeoutChan():
			if err = c.checkTimeout(); err != nil {
				return
			}
		}
	}
}

func (c *TCPConnection) batchWriteMsg(msg interface{}) error {
	var ok bool
	l := len(c.sendChan)
	msgList := make([]interface{}, l+1)
	msgList = append(msgList, msg)
	for i := 0; i < l; i++ {
		msg, ok = <-c.sendChan
		if !ok {
			break
		}
		msgList = append(msgList, msg)
	}
	return c.write(c.conn, msgList...)
}

func (c *TCPConnection) Close(err error) (w error) {
	if !c.closeBase() {
		return ErrConnectionClosed
	}

	glog.Info("TCP连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))

	if c.conn != nil {
		if w = c.conn.Close(); w != nil {
			return
		}
	}
	if c.baseConnection != nil {
		if w = c.baseConnection.Close(c, err); w != nil {
			return
		}
	}

	return
}

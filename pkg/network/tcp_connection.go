package network

import (
	"context"
	"gas/pkg/glog"
	"gas/pkg/lib/buffer"
	"io"
	"net"

	"go.uber.org/zap"
)

type TCPConnection struct {
	*baseConnection // 嵌入基类
	conn            *net.TCPConn
	server          *TCPServer // 所属服务器
	tmpBuf          []byte
	buffer          buffer.IBuffer
}

func newTCPConnection(ctx context.Context, conn *net.TCPConn, typ ConnectionType, options *Options) *TCPConnection {
	setConOptions(options, conn)

	base := initBaseConnection(ctx, typ, conn.LocalAddr(), conn.RemoteAddr(), options)
	tcpConn := &TCPConnection{
		baseConnection: base,
		tmpBuf:         make([]byte, options.ReadBufSize),
		buffer:         buffer.New(options.ReadBufSize),
		conn:           conn,
	}
	return tcpConn
}

func (c *TCPConnection) readLoop() {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.onConnect(c); err != nil {
		return
	}
	for !c.IsStop() {
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
		if err != nil {
			return err
		}
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
		_ = c.batchWriteMsg(nil)
		_ = c.conn.Close()
		_ = c.Close(err)
	}()

	for !c.IsStop() {
		select {
		case <-c.ctx.Done():
			return
		case msg := <-c.sendChan:
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
	var msgList = []interface{}{msg}
	for i := 0; i < len(c.sendChan); i++ {
		msg = <-c.sendChan
		msgList = append(msgList, msg)
	}
	return c.write(c.conn, msgList...)
}

func (c *TCPConnection) Close(err error) (w error) {
	if !c.Stop() {
		return ErrConnectionClosed
	}

	glog.Info("TCP连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))

	c.baseConnection.Close(c, err)
	return
}

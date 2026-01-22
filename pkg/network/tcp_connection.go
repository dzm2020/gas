package network

import (
	"context"
	"io"
	"net"

	"github.com/dzm2020/gas/pkg/glog"
	"go.uber.org/zap"
)

type TCPConnection struct {
	*baseConn // 嵌入基类
	conn      *net.TCPConn
	tmpBuf    []byte
}

func newTCPConnection(ctx context.Context, conn *net.TCPConn, typ ConnType, options *Options) *TCPConnection {
	base := newBaseConn(ctx, "tcp", typ, conn, options)
	tcpConn := &TCPConnection{
		baseConn: base,
		tmpBuf:   make([]byte, options.ReadBufSize),
		conn:     conn,
	}
	return tcpConn
}

func (c *TCPConnection) readLoop() {
	var err error
	var n int
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.onConnect(c); err != nil {
		return
	}
	for !c.IsStop() {
		n, err = c.conn.Read(c.tmpBuf)
		if err != nil {
			return
		}
		if n == 0 {
			err = io.EOF
			return
		}
		_, err = c.process(c, c.tmpBuf[:n])
		if err != nil {
			return
		}
	}
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

	c.baseConn.Close(c, err)
	return
}

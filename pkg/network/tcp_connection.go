package network

import (
	"context"
	"errors"
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
	base := newBaseConn(ctx, "tcp", typ, conn, conn.RemoteAddr(), options)
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
			// 区分 EOF 和其他错误
			if err == io.EOF {
				return
			}
			// 对于其他错误，记录日志后返回
			if !errors.Is(err, net.ErrClosed) {
				glog.Error("TCP连接读取错误", zap.Int64("connectionId", c.ID()), zap.Error(err))
			}
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
		case msg, ok := <-c.sendChan:
			if !ok {
				// channel 已关闭
				return
			}
			if err = c.batchWriteMsg(msg); err != nil {
				return
			}
		}
	}
}

func (c *TCPConnection) batchWriteMsg(msg interface{}) error {
	const maxBatchSize = 100 // 限制批量写入的最大消息数，避免内存占用过高
	var msgList = []interface{}{msg}
	batchCount := 0
	for len(c.sendChan) > 0 && batchCount < maxBatchSize-1 {
		select {
		case msg := <-c.sendChan:
			msgList = append(msgList, msg)
			batchCount++
		default:
			break
		}
	}
	return c.write(c.conn, msgList...)
}

func (c *TCPConnection) Close(err error) (w error) {
	if !c.Stop() {
		return ErrConnectionClosed
	}

	c.baseConn.Close(c, err)

	glog.Info("TCP连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))
	return
}

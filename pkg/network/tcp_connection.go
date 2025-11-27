package network

import (
	"context"
	"errors"
	"gas/pkg/lib/buffer"
	"gas/pkg/lib/glog"
	"gas/pkg/lib/workers"
	"net"
	"time"

	"go.uber.org/zap"
)

type TCPConnection struct {
	*baseConnection // 嵌入基类
	conn            net.Conn
	server          *TCPServer  // 所属服务器
	sendChan        chan []byte // 发送队列（读写分离核心）
	buf             []byte      // 粘包缓冲（未解析完整的消息数据）
	buffer          buffer.IBuffer
}

func newTCPConnection(conn net.Conn, typ ConnectionType, options *Options) *TCPConnection {
	tcpConn := &TCPConnection{
		baseConnection: initBaseConnection(typ, options),
		sendChan:       make(chan []byte, options.sendChanSize),
		buf:            make([]byte, options.readBufSize),
		buffer:         buffer.New(options.readBufSize),
		conn:           conn,
	}
	AddConnection(tcpConn)

	workers.Go(func(ctx context.Context) {
		tcpConn.readLoop(ctx)
	})

	workers.Go(func(ctx context.Context) {
		tcpConn.writeLoop(ctx)
	})
	glog.Infof("tcp connection open %v local:%v remote:%v typ:%v", tcpConn.ID(), tcpConn.LocalAddr(), tcpConn.RemoteAddr(), tcpConn.Type())
	return tcpConn
}

func (c *TCPConnection) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *TCPConnection) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// Send 发送消息（线程安全）
func (c *TCPConnection) Send(msg interface{}) error {
	if err := c.checkClosed("tcp"); err != nil {
		return err
	}
	// 编码消息
	data, err := c.codec.Encode(msg)
	if err != nil {
		return err
	}
	select {
	case c.sendChan <- data:
	default:
		glog.Error("tcp connection send chan full", zap.Int64("conn_id", c.ID()))
		return errors.New("tcp send queue full")
	}
	return nil
}

func (c *TCPConnection) readLoop(ctx context.Context) {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.handler.OnConnect(c); err != nil {
		glog.Error("tcp connection OnConnect error", zap.Int64("conn_id", c.ID()), zap.Error(err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			return // 主动关闭，无错误
		case <-c.closeChan:
			return
		default:
			if err = c.read(); err != nil {
				return
			}
		}
	}
}

func (c *TCPConnection) read() error {
	n, readErr := c.conn.Read(c.buf)
	if readErr != nil {
		return readErr
	}
	if n == 0 {
		return errors.New("tcp read zero bytes")
	}

	if _, readErr = c.buffer.Write(c.buf[:n]); readErr != nil {
		return readErr
	}

	// 循环解码（处理粘包，可能一次读取多个消息）
	for c.buffer.Len() > 0 {
		pn, err := c.process(c, c.buffer.Bytes())
		if pn == 0 {
			break // 数据不完整，等待下一次读取
		}
		_ = c.buffer.Skip(pn)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *TCPConnection) writeLoop(ctx context.Context) {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	var timeoutChan <-chan time.Time
	if c.timeoutTicker != nil {
		timeoutChan = c.timeoutTicker.C
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closeChan:
			return
		case data, ok := <-c.sendChan:
			if !ok {
				return // 通道已关闭
			}
			_, err = c.conn.Write(data)
			if err != nil {
				return
			}
		case <-timeoutChan:
			if c.isTimeout() {
				err = errors.New("tcp connection timeout")
				return
			}
		}
	}
}

func (c *TCPConnection) Close(err error) error {
	if !c.closeBase() {
		return errors.New("tcp connection already closed")
	}

	close(c.closeChan)

	workers.Go(func(ctx context.Context) {
		_ = c.conn.Close()
		_ = c.baseConnection.Close(c, err)
	})

	glog.Info("tcp connection close", zap.Int64("conn_id", c.ID()), zap.Error(err))
	return nil
}

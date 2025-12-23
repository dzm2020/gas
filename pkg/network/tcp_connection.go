package network

import (
	"context"
	"gas/pkg/glog"
	"gas/pkg/lib"
	"gas/pkg/lib/grs"
	"net"
	"time"

	"go.uber.org/zap"
)

type TCPConnection struct {
	*baseConnection // 嵌入基类
	conn            net.Conn
	server          *TCPServer  // 所属服务器
	sendChan        chan []byte // 发送队列（读写分离核心）
	tmpBuf          []byte
	buffer          lib.IBuffer
}

func newTCPConnection(conn net.Conn, typ ConnectionType, options *Options) *TCPConnection {
	tcpConn := &TCPConnection{
		baseConnection: initBaseConnection(typ, options),
		sendChan:       make(chan []byte, options.sendChanSize),
		tmpBuf:         make([]byte, options.readBufSize),
		buffer:         lib.New(options.readBufSize),
		conn:           conn,
	}
	AddConnection(tcpConn)

	grs.Go(func(ctx context.Context) {
		tcpConn.readLoop(ctx)
	})

	grs.Go(func(ctx context.Context) {
		tcpConn.writeLoop(ctx)
	})

	glog.Info("创建TCP连接", zap.Int64("connectionId", tcpConn.ID()),
		zap.String("localAddr", tcpConn.LocalAddr().String()),
		zap.String("remoteAddr", tcpConn.RemoteAddr().String()))
	return tcpConn
}

func (c *TCPConnection) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *TCPConnection) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// Send 发送消息（线程安全）
func (c *TCPConnection) Send(msg interface{}) error {
	if err := c.checkClosed(); err != nil {
		return err
	}
	// 编码消息
	data, err := c.codec.Encode(msg)
	if err != nil {
		glog.Error("TCP发送消息编码失败", zap.Int64("connectionId", c.ID()), zap.Error(err))
		return err
	}
	select {
	case c.sendChan <- data:
	default:
		glog.Error("TCP发送消息失败channel已满", zap.Int64("connectionId", c.ID()))
		return ErrTCPSendQueueFull
	}
	return nil
}

func (c *TCPConnection) readLoop(ctx context.Context) {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.handler.OnConnect(c); err != nil {
		glog.Error("TCP连接回调错误", zap.Int64("connectionId", c.ID()), zap.Error(err))
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
	n, readErr := c.conn.Read(c.tmpBuf)

	if n == 0 {
		return ErrTCPReadZeroBytes
	}

	if readErr != nil {
		glog.Error("TCP读取消息失败", zap.Int64("connectionId", c.ID()), zap.Error(readErr))
		return readErr
	}

	if _, readErr = c.buffer.Write(c.tmpBuf[:n]); readErr != nil {
		glog.Error("TCP读取消息失败", zap.Int64("connectionId", c.ID()), zap.Error(readErr))
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
			glog.Error("TCP处理消息失败", zap.Int64("connectionId", c.ID()), zap.Error(err))
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
				glog.Error("TCP写入消息失败", zap.Int64("connectionId", c.ID()), zap.Error(err))
				return
			}
		case <-timeoutChan:
			if c.isTimeout() {
				err = ErrTCPConnectionKeepAlive
				glog.Warn("TCP心跳超时", zap.Int64("connectionId", c.ID()), zap.Error(err))
				return
			}
		}
	}
}

func (c *TCPConnection) Close(err error) error {
	if !c.closeBase() {
		return ErrTCPConnectionClosed
	}

	grs.Go(func(ctx context.Context) {
		_ = c.conn.Close()
		_ = c.baseConnection.Close(c, err)
	})

	glog.Info("TCP连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))
	return nil
}

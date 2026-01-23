package network

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/buffer"
	"github.com/dzm2020/gas/pkg/lib/netutil"
	"github.com/dzm2020/gas/pkg/lib/stopper"
	"go.uber.org/zap"
)

// newBaseConn 初始化基类连接
func newBaseConn(ctx context.Context, network string,
	typ ConnType, conn net.Conn, remoteAddr net.Addr, options *Options) *baseConn {
	bc := &baseConn{
		id:          genConnID(),
		network:     network,
		options:     options,
		conn:        conn,
		remoteAddr:  remoteAddr,
		lastActive:  time.Now(),
		typ:         typ,
		sendChan:    make(chan interface{}, options.SendChanSize),
		writeBuffer: buffer.New(options.SendBufferSize),
		readBuffer:  buffer.New(options.ReadBufSize),
	}
	bc.ctx, bc.cancel = context.WithCancel(ctx)
	glog.Info("新建网络连接", zap.Int64("connectionId", bc.ID()),
		zap.String("network", bc.Network()),
		zap.Int("typ", bc.Type()),
		zap.String("localAddr", bc.LocalAddr()),
		zap.String("remoteAddr", bc.RemoteAddr()))
	return bc
}

// baseConn 连接基类，包含所有连接的通用逻辑
type baseConn struct {
	stopper.Stopper
	id          int64 // 连接唯一ID
	network     string
	handler     IHandler
	options     *Options  // 选项
	conn        net.Conn  // 原生连接
	lastActive  time.Time // 最后活动时间（用于超时检测）
	typ         ConnType  // 连接类型
	user        interface{}
	sendChan    chan interface{}
	ctx         context.Context
	cancel      context.CancelFunc
	remoteAddr  net.Addr
	writeBuffer buffer.IBuffer
	readBuffer  buffer.IBuffer
}

// ID 返回连接的唯一ID
func (b *baseConn) ID() int64 {
	return b.id
}

func (b *baseConn) Type() ConnType {
	return b.typ
}
func (b *baseConn) Network() string {
	return b.network
}
func (b *baseConn) LocalAddr() string {
	return b.conn.LocalAddr().String()
}
func (b *baseConn) RemoteAddr() string {
	return b.remoteAddr.String()
}
func (b *baseConn) Context() interface{} {
	return b.ctx
}
func (b *baseConn) SetContext(ctx interface{}) {
	b.user = ctx
}
func (b *baseConn) SetReadBuffer(bytes int) error {
	return netutil.SetRcvBuffer(b.conn, bytes)
}
func (b *baseConn) SetWriteBuffer(bytes int) error {
	return netutil.SetSndBuffer(b.conn, bytes)
}
func (b *baseConn) SetLinger(enable bool, sec int) error {
	return netutil.SetTCPLinger(b.conn, enable, sec)
}

func (b *baseConn) SetNoDelay(noDelay bool) error {
	return netutil.SetTCPNoDelay(b.conn, noDelay)
}
func (b *baseConn) SetTCPKeepAlive(enable bool, period time.Duration) error {
	return netutil.SetTCPKeepAlive(b.conn, enable, period)
}
func (b *baseConn) SetHandler(handler IHandler) {
	b.handler = handler
}

func (b *baseConn) heartLoop(connection IConnection) {
	var err error
	timeout := b.options.HeartTimeout
	ticker := time.NewTicker(timeout / 2)
	defer func() {
		ticker.Stop()
		if closeErr := connection.Close(err); closeErr != nil && !errors.Is(closeErr, ErrConnectionClosed) {
			glog.Error("心跳循环关闭连接时出错", zap.Int64("connectionId", connection.ID()), zap.Error(closeErr))
		}
	}()

	for !b.IsStop() {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			if time.Since(b.lastActive) > timeout {
				err = ErrConnHeartTimeout
				return
			}
		}
	}
}

func (b *baseConn) encode(msg interface{}) (bin []byte, err error) {
	return b.options.Codec.Encode(msg)
}

func (b *baseConn) onConnect(connection IConnection) error {
	return b.handler.OnConnect(connection)
}

func (b *baseConn) OnMessage(conn IConnection, msg interface{}) error {
	return b.handler.OnMessage(conn, msg)
}

func (b *baseConn) OnClose(conn IConnection, err error) {
	b.handler.OnClose(conn, err)
}

// Send 发送消息（线程安全）
func (b *baseConn) Send(msg interface{}) error {
	if b.IsStop() {
		return ErrConnectionClosed
	}
	if msg == nil {
		return nil
	}
	select {
	case b.sendChan <- msg:
	default:
		return ErrChannelFull
	}
	return nil
}

// write 批量写入多个消息到指定的 Writer
// 将多个消息编码后合并写入，用于批量发送场景
func (b *baseConn) write(c io.Writer, msgList ...interface{}) error {
	if len(msgList) == 0 {
		return nil // 没有消息需要写入，直接返回
	}
	for _, msg := range msgList {
		if msg == nil {
			continue
		}
		bytes, err := b.encode(msg)
		if err != nil {
			return err
		}
		// write buffer
		if _, err = b.writeBuffer.Write(bytes); err != nil {
			return err
		}
	}
	// write socket
	for b.writeBuffer.Len() > 0 {
		n, err := c.Write(b.writeBuffer.Bytes())
		if err != nil {
			return err
		}
		_ = b.writeBuffer.Skip(n)
	}
	return nil
}

func (b *baseConn) process(connection IConnection, data []byte) (int, error) {
	b.lastActive = time.Now()
	_, _ = b.readBuffer.Write(data)
	msg, n, err := b.options.Codec.Decode(b.readBuffer.Bytes())
	if err != nil {
		return n, err
	}
	_ = b.readBuffer.Skip(n)
	return n, b.OnMessage(connection, msg)
}

func (b *baseConn) Close(connection IConnection, err error) {
	if !b.Stop() {
		return
	}
	b.OnClose(connection, err)
	RemoveConnection(connection)
	b.cancel()
	return
}

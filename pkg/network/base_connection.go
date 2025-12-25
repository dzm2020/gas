package network

import (
	"errors"
	"gas/pkg/glog"
	"gas/pkg/lib/buffer"
	"io"
	"net"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

var (
	ErrCodecIsNil      = errors.New("codec is nil")
	ErrHandlerIsNil    = errors.New("handler is nil")
	ErrBinLengthIsZero = errors.New("bin length is zero")
)

// baseConnection 连接基类，包含所有连接的通用逻辑
type baseConnection struct {
	id                    int64         // 连接唯一ID
	timeoutTicker         *time.Ticker  // 心跳超时定时器
	lastActive            time.Time     // 最后活动时间（用于超时检测）
	timeout               time.Duration // 超时时间
	handler               IHandler
	codec                 ICodec
	typ                   ConnectionType
	ctx                   interface{}
	sendChan              chan interface{}
	closed                atomic.Bool // 关闭状态（原子操作）
	localAddr, remoteAddr net.Addr
}

// initBaseConnection 初始化基类连接
func initBaseConnection(typ ConnectionType, localAddr, remoteAddr net.Addr, options *Options) *baseConnection {
	bc := &baseConnection{
		id:         generateConnID(),
		lastActive: time.Now(),
		timeout:    options.keepAlive,
		handler:    options.handler,
		codec:      options.codec,
		typ:        typ,
		sendChan:   make(chan interface{}),
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}
	// 只有当 keepAlive > 0 时才创建 ticker
	if options.keepAlive > 0 {
		bc.timeoutTicker = time.NewTicker(options.keepAlive / 2)
	}

	glog.Info("创建连接", zap.Int64("connectionId", bc.ID()),
		zap.String("localAddr", bc.LocalAddr()),
		zap.String("remoteAddr", bc.RemoteAddr()))

	return bc
}

// ID 返回连接的唯一ID
func (b *baseConnection) ID() int64 {
	return b.id
}

func (b *baseConnection) Type() ConnectionType {
	return b.typ
}

func (b *baseConnection) LocalAddr() string {
	if b.localAddr == nil {
		return ""
	}
	return b.localAddr.String()
}

func (b *baseConnection) RemoteAddr() string {
	if b.remoteAddr == nil {
		return ""
	}
	return b.remoteAddr.String()
}

func (b *baseConnection) Context() interface{} {
	return b.ctx
}

func (b *baseConnection) SetContext(ctx interface{}) {
	b.ctx = ctx
}

// isClosed 检查连接是否已关闭
func (b *baseConnection) isClosed() bool {
	return b.closed.Load()
}

// closeBase 关闭基类连接（通用逻辑）
// 返回是否成功关闭（false表示已经关闭过）
func (b *baseConnection) closeBase() bool {
	return b.closed.CompareAndSwap(false, true)
}

// checkClosed 检查连接是否已关闭，如果已关闭返回错误
func (b *baseConnection) checkClosed() error {
	if b.closed.Load() {
		return ErrConnectionClosed
	}
	return nil
}

func (b *baseConnection) IsClosed() bool {
	return b.closed.Load()
}

func (b *baseConnection) updateLastActive() {
	b.lastActive = time.Now()
}

func (b *baseConnection) getLastActive() time.Time {
	return b.lastActive
}

func (b *baseConnection) getTimeoutChan() <-chan time.Time {
	if b.timeoutTicker != nil {
		return b.timeoutTicker.C
	}
	return nil
}

func (b *baseConnection) checkTimeout() error {
	if b.timeout <= 0 {
		return nil // keepAlive 为 0 表示不检测超时
	}
	if time.Since(b.getLastActive()) > b.timeout {
		return ErrConnectionKeepAlive
	}
	return nil
}

func (b *baseConnection) encode(msg interface{}) (bin []byte, err error) {
	if b.codec == nil {
		err = ErrCodecIsNil
		return
	}
	bin, err = b.codec.Encode(msg)
	if err != nil {
		return
	}
	if len(bin) <= 0 {
		err = ErrBinLengthIsZero
		return
	}
	return
}

func (b *baseConnection) onConnect(connection IConnection) error {
	if b.handler == nil {
		glog.Error("连接回调错误", zap.Int64("connectionId", connection.ID()), zap.Error(ErrHandlerIsNil))
		return ErrHandlerIsNil
	}
	if err := b.handler.OnConnect(connection); err != nil {
		glog.Error("连接回调错误", zap.Int64("connectionId", connection.ID()), zap.Error(err))
		return err
	}
	glog.Debug("连接回调成功", zap.Int64("connectionId", connection.ID()))
	return nil
}

// Send 发送消息（线程安全）
func (b *baseConnection) Send(msg interface{}) error {
	if err := b.checkClosed(); err != nil {
		return err
	}
	select {
	case b.sendChan <- msg:
	default:
		glog.Error("发送消息失败channel已满", zap.Int64("connectionId", b.ID()))
		return ErrSendQueueFull
	}
	return nil
}

// write 批量写入多个消息到指定的 Writer
// 将多个消息编码后合并写入，用于批量发送场景
func (b *baseConnection) write(c io.Writer, msgList ...interface{}) error {
	if c == nil {
		return errors.New("writer is nil")
	}
	if len(msgList) == 0 {
		return nil // 没有消息需要写入，直接返回
	}
	// 使用缓冲区合并多个消息
	buf := buffer.New(4096)
	for _, msg := range msgList {
		// 使用 encode 方法，保持与 Send 方法的一致性
		bytes, err := b.encode(msg)
		if err != nil {
			glog.Error("批量写入消息编码失败", zap.Int64("connectionId", b.ID()), zap.Error(err))
			return err
		}
		// 写入缓冲区
		if _, err := buf.Write(bytes); err != nil {
			glog.Error("批量写入消息到缓冲区失败", zap.Int64("connectionId", b.ID()), zap.Error(err))
			return err
		}
	}
	// 一次性写入所有数据
	if _, err := c.Write(buf.Bytes()); err != nil {
		glog.Error("批量写入消息失败", zap.Int64("connectionId", b.ID()), zap.Error(err))
		return err
	}
	return nil
}

func (b *baseConnection) process(connection IConnection, data []byte) (int, error) {
	b.updateLastActive()
	codec := b.codec
	if codec == nil {
		return 0, ErrCodecIsNil
	}
	msg, n, err := codec.Decode(data)
	if err != nil {
		return n, err
	}
	handler := b.handler
	if handler == nil {
		return n, ErrHandlerIsNil
	}
	return n, handler.OnMessage(connection, msg)
}

func (b *baseConnection) Close(connection IConnection, err error) (w error) {
	if b.sendChan != nil {
		close(b.sendChan)
		b.sendChan = nil
	}

	if connection != nil {
		RemoveConnection(connection)
		connection = nil
	}
	if b.handler != nil {
		w = b.handler.OnClose(connection, err)
	}
	if b.timeoutTicker != nil {
		b.timeoutTicker.Stop()
		b.timeoutTicker = nil
	}
	return w
}

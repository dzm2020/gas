package network

import (
	"sync/atomic"
	"time"
)

// baseConnection 连接基类，包含所有连接的通用逻辑
type baseConnection struct {
	id            int64 // 连接唯一ID
	timeoutTicker *time.Ticker
	closed        atomic.Bool   // 关闭状态（原子操作）
	lastActive    time.Time     // 最后活动时间（用于超时检测）
	timeout       time.Duration // 超时时间
	closeChan     chan struct{} // 关闭信号
	handler       IHandler
	codec         ICodec
	typ           ConnectionType
	ctx           interface{}
}

// initBaseConnection 初始化基类连接
func initBaseConnection(typ ConnectionType, options *Options) *baseConnection {
	bc := &baseConnection{
		id:         generateConnID(),
		lastActive: time.Now(),
		timeout:    options.keepAlive,
		closeChan:  make(chan struct{}),
		handler:    options.handler,
		codec:      options.codec,
		typ:        typ,
	}
	// 只有当 keepAlive > 0 时才创建 ticker
	if options.keepAlive > 0 {
		bc.timeoutTicker = time.NewTicker(options.keepAlive / 2)
	}
	return bc
}

// ID 返回连接的唯一ID
func (b *baseConnection) ID() int64 {
	return b.id
}

// isClosed 检查连接是否已关闭
func (b *baseConnection) isClosed() bool {
	return b.closed.Load()
}

func (b *baseConnection) updateLastActive() {
	b.lastActive = time.Now()
}

func (b *baseConnection) getLastActive() time.Time {
	return b.lastActive
}

// closeBase 关闭基类连接（通用逻辑）
// 返回是否成功关闭（false表示已经关闭过）
func (b *baseConnection) closeBase() bool {
	return b.closed.CompareAndSwap(false, true)
}

// closeChanSignal 返回关闭信号通道
func (b *baseConnection) closeChanSignal() chan struct{} {
	return b.closeChan
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

func (b *baseConnection) Type() ConnectionType {
	return b.typ
}

func (b *baseConnection) isTimeout() bool {
	if b.timeout <= 0 {
		return false // keepAlive 为 0 表示不检测超时
	}
	return time.Since(b.getLastActive()) > b.timeout
}

func (b *baseConnection) process(connection IConnection, data []byte) (int, error) {
	b.updateLastActive()
	msg, n, err := b.codec.Decode(data)
	if err != nil {
		return n, err
	}
	return n, b.handler.OnMessage(connection, msg)
}

func (b *baseConnection) Close(connection IConnection, err error) error {
	RemoveConnection(connection)
	close(b.closeChan)
	_ = b.handler.OnClose(connection, err)
	b.timeoutTicker.Stop()
	return err
}

func (b *baseConnection) Context() interface{} {
	return b.ctx
}

func (b *baseConnection) SetContext(ctx interface{}) {
	b.ctx = ctx
}

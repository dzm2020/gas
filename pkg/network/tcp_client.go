package network

import (
	"net"
	"sync/atomic"
	"time"
)

// ------------------------------ TCP客户端 ------------------------------

type TCPClient struct {
	*TCPConnection // 嵌入TCP连接
	options        *Options
	addr           string      // 服务器地址
	connected      atomic.Bool // 连接状态
	timeout        time.Duration
}

// NewTCPClient 创建TCP客户端
func NewTCPClient(addr string, timeout time.Duration, option ...Option) *TCPClient {
	options := loadOptions(option...)
	client := &TCPClient{
		options: options,
		addr:    addr,
		timeout: timeout,
	}
	return client
}

// Connect 连接到服务器
func (c *TCPClient) Connect() error {
	if c.connected.Load() {
		return ErrTCPClientAlreadyConnected
	}

	conn, err := net.DialTimeout("tcp", c.addr, c.timeout)
	if err != nil {
		return err
	}

	// 创建TCP连接
	c.TCPConnection = newTCPConnection(conn, Connect, c.options)
	c.connected.Store(true)
	return nil
}

// Disconnect 断开连接
func (c *TCPClient) Disconnect() error {
	if !c.connected.Load() {
		return ErrTCPClientNotConnected
	}

	if c.TCPConnection != nil {
		_ = c.TCPConnection.Close(nil)
		c.TCPConnection = nil
	}
	c.connected.Store(false)
	return nil
}

// Send 发送消息（直接使用嵌入的TCPConnection的方法）
func (c *TCPClient) Send(msg interface{}) error {
	if !c.connected.Load() || c.TCPConnection == nil {
		return ErrTCPClientNotConnected
	}
	return c.TCPConnection.Send(msg)
}

// Connection 获取当前连接（实现兼容性）
func (c *TCPClient) Connection() IConnection {
	return c.TCPConnection
}

// IsConnected 检查是否已连接
func (c *TCPClient) IsConnected() bool {
	return c.connected.Load() && c.TCPConnection != nil && !c.TCPConnection.isClosed()
}

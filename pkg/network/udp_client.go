package network

import (
	"errors"
	"gas/pkg/utils/glog"
	"gas/pkg/utils/workers"
	"net"
	"sync/atomic"

	"go.uber.org/zap"
)

// ------------------------------ UDP客户端 ------------------------------

type UDPClient struct {
	options   *Options
	conn      *net.UDPConn
	addr      *net.UDPAddr  // 服务器地址
	connected atomic.Bool   // 连接状态
	closeChan chan struct{} // 关闭信号
	readChan  chan []byte   // 接收数据通道
}

// NewUDPClient 创建UDP客户端
func NewUDPClient(serverAddr string, option ...Option) (*UDPClient, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, err
	}

	// 绑定本地地址（使用随机端口）
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}

	client := &UDPClient{
		options:   loadOptions(option...),
		conn:      conn,
		addr:      udpAddr,
		closeChan: make(chan struct{}),
		readChan:  make(chan []byte, 100),
	}

	// 启动接收协程
	workers.Submit(func() {
		client.readLoop()
	}, func(err interface{}) {
		glog.Error("udp client readLoop panic", zap.Any("panic", err))
	})

	// 启动处理协程
	workers.Submit(func() {
		client.processLoop()
	}, func(err interface{}) {
		glog.Error("udp client processLoop panic", zap.Any("panic", err))
	})

	client.connected.Store(true)
	return client, nil
}

// Send 发送消息到服务器
func (c *UDPClient) Send(msg interface{}) error {
	if !c.connected.Load() {
		return errors.New("udp client not connected")
	}

	if c.options.codec == nil {
		return errors.New("codec is nil")
	}

	data, err := c.options.codec.Encode(msg)
	if err != nil {
		return err
	}

	_, err = c.conn.WriteToUDP(data, c.addr)
	return err
}

// Close 关闭UDP客户端
func (c *UDPClient) Close() error {
	if !c.connected.CompareAndSwap(true, false) {
		return errors.New("udp client already closed")
	}

	close(c.closeChan)
	if c.conn != nil {
		_ = c.conn.Close()
	}
	close(c.readChan)
	return nil
}

// LocalAddr 本地地址
func (c *UDPClient) LocalAddr() net.Addr {
	if c.conn == nil {
		return nil
	}
	return c.conn.LocalAddr()
}

// RemoteAddr 远程地址（服务器地址）
func (c *UDPClient) RemoteAddr() net.Addr {
	return c.addr
}

// IsConnected 检查是否已连接
func (c *UDPClient) IsConnected() bool {
	return c.connected.Load()
}

// readLoop 接收数据循环
func (c *UDPClient) readLoop() {
	readBuf := make([]byte, c.options.readBufSize)
	for {
		select {
		case <-c.closeChan:
			return
		default:
			n, _, err := c.conn.ReadFromUDP(readBuf)
			if err != nil {
				if !c.connected.Load() {
					return
				}
				glog.Error("udp client read failed", zap.Error(err))
				continue
			}
			if n == 0 {
				continue
			}

			// 复制数据到通道
			data := make([]byte, n)
			copy(data, readBuf[:n])
			select {
			case c.readChan <- data:
			case <-c.closeChan:
				return
			}
		}
	}
}

// processLoop 处理接收到的数据
func (c *UDPClient) processLoop() {
	if c.options.handler == nil || c.options.codec == nil {
		return
	}

	// 创建虚拟连接用于回调
	conn := &UDPClientConnection{
		client: c,
	}

	// 首次连接回调
	c.options.handler.OnConnect(conn)

	for {
		select {
		case <-c.closeChan:
			return
		case data, ok := <-c.readChan:
			if !ok {
				return
			}

			// 解码消息
			msg, _, err := c.options.codec.Decode(data)
			if err != nil {
				glog.Error("udp client decode failed", zap.Error(err))
				continue
			}

			if msg != nil {
				// 回调OnMessage
				if err := c.options.handler.OnMessage(conn, msg); err != nil {
					glog.Error("udp client OnMessage error", zap.Error(err))
				}
			}
		}
	}
}

// UDPClientConnection UDP客户端连接包装（实现IConnection接口）
type UDPClientConnection struct {
	client *UDPClient
	id     int64
}

func (c *UDPClientConnection) Type() ConnectionType {
	return Connect
}

func (c *UDPClientConnection) IsClosed() bool {
	return false
}

func (c *UDPClientConnection) ID() int64 {
	if c.id == 0 {
		c.id = generateConnID()
	}
	return c.id
}

func (c *UDPClientConnection) Send(msg interface{}) error {
	return c.client.Send(msg)
}

func (c *UDPClientConnection) Close(err error) error {
	return c.client.Close()
}

func (c *UDPClientConnection) LocalAddr() net.Addr {
	return c.client.LocalAddr()
}

func (c *UDPClientConnection) RemoteAddr() net.Addr {
	return c.client.RemoteAddr()
}

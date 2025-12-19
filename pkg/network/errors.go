package network

import (
	"errors"
	"fmt"
)

// 连接相关错误
var (
	// ErrConnectionClosed 连接已关闭
	ErrConnectionClosed = errors.New("connection closed")
	// ErrTCPConnectionClosed TCP连接已关闭
	ErrTCPConnectionClosed = errors.New("tcp connection already closed")
	// ErrUDPConnectionClosed UDP连接已关闭
	ErrUDPConnectionClosed = errors.New("udp connection already closed")
	// ErrTCPSendQueueFull TCP发送队列已满
	ErrTCPSendQueueFull = errors.New("tcp send queue full")
	// ErrTCPReadZeroBytes TCP读取到0字节
	ErrTCPReadZeroBytes = errors.New("tcp read zero bytes")
	// ErrTCPConnectionKeepAlive TCP连接超时
	ErrTCPConnectionKeepAlive = errors.New("tcp connection keepAlive")
	// ErrUDPConnectionKeepAlive UDP连接超时
	ErrUDPConnectionKeepAlive = errors.New("udp connection keepAlive")
)

// 服务器相关错误
var (
	// ErrTCPServerAlreadyRunning TCP服务器已在运行
	ErrTCPServerAlreadyRunning = errors.New("tcp server already running")
	// ErrTCPServerNotRunning TCP服务器未运行
	ErrTCPServerNotRunning = errors.New("tcp server not running")
	// ErrUDPServerAlreadyRunning UDP服务器已在运行
	ErrUDPServerAlreadyRunning = errors.New("udp server already running")
	// ErrUDPServerNotRunning UDP服务器未运行
	ErrUDPServerNotRunning = errors.New("udp server not running")
)

// 客户端相关错误
var (
	// ErrTCPClientAlreadyConnected TCP客户端已连接
	ErrTCPClientAlreadyConnected = errors.New("tcp client already connected")
	// ErrTCPClientNotConnected TCP客户端未连接
	ErrTCPClientNotConnected = errors.New("tcp client not connected")
)

// ErrUnsupportedProtocol 不支持的协议错误
func ErrUnsupportedProtocol(proto string) error {
	return fmt.Errorf("proto: %s is not support", proto)
}














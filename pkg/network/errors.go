package network

import (
	"errors"
	"fmt"
)

// 连接相关错误
var (
	ErrConnectionClosed    = errors.New("connection closed")
	ErrSendQueueFull       = errors.New("send queue full")
	ErrConnectionKeepAlive = errors.New("connection keepAlive")
	ErrListenerIsNil       = errors.New("listener is nil")
)

// 客户端相关错误
var (
	// ErrTCPClientAlreadyConnected TCP客户端已连接
	ErrTCPClientAlreadyConnected = errors.New("tcp client already connected")
	// ErrTCPClientNotConnected TCP客户端未连接
	ErrTCPClientNotConnected = errors.New("tcp client not connected")
)

// WebSocket 相关错误
var (

	// ErrWebSocketConnectionKeepAlive WebSocket连接超时
	ErrWebSocketConnectionKeepAlive = errors.New("websocket connection keepAlive")
	// ErrWebSocketServerAlreadyRunning WebSocket服务器已在运行
	ErrWebSocketServerAlreadyRunning = errors.New("websocket server already running")
	// ErrWebSocketServerNotRunning WebSocket服务器未运行
	ErrWebSocketServerNotRunning = errors.New("websocket server not running")
)

// ErrUnsupportedProtocol 不支持的协议错误
func ErrUnsupportedProtocol(proto string) error {
	return fmt.Errorf("proto: %s is not support", proto)
}

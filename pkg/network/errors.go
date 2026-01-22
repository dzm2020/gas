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
	ErrConnHeartTimeout    = errors.New("heart timeout")
)

// ErrUnsupportedProtocol 不支持的协议错误
func ErrUnsupportedProtocol(proto string) error {
	return fmt.Errorf("proto: %s is not support", proto)
}

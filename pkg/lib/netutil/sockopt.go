package netutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

// ==================== 通用工具函数：获取 RawConn ====================

// getRawConnFromConn 从 Conn 获取 syscall.RawConn
func getRawConnFromConn(conn net.Conn) (syscall.RawConn, error) {
	var rawConn syscall.RawConn
	var err error

	switch c := conn.(type) {
	case *net.TCPConn:
		rawConn, err = c.SyscallConn()
		if err != nil {
			return nil, fmt.Errorf("get TCP conn raw conn failed: %w", err)
		}
	case *net.UDPConn:
		rawConn, err = c.SyscallConn()
		if err != nil {
			return nil, fmt.Errorf("get UDP conn raw conn failed: %w", err)
		}
	default:
		return nil, errors.New("unsupported conn type")
	}
	return rawConn, nil
}

// setReuseAddr 设置 SO_REUSEADDR 选项（跨平台支持）
func setReuseAddr(fd uintptr) error {
	return setSockOptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}

// ==================== 对外暴露的核心接口 ====================

// ListenConfig 封装了 socket 选项配置的 net.ListenConfig
type ListenConfig struct {
	ReuseAddr bool
	ReusePort bool
}

// NewListenConfig 创建新的 ListenConfig，默认启用 ReuseAddr
func NewListenConfig() *ListenConfig {
	return &ListenConfig{
		ReuseAddr: true,
		ReusePort: false, // 默认不启用，因为仅 Linux 支持
	}
}

// Control 返回 net.ListenConfig 使用的 Control 函数
func (lc *ListenConfig) Control() func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var controlErr error
		err := c.Control(func(fd uintptr) {
			// 设置 SO_REUSEADDR（跨平台支持）
			if lc.ReuseAddr {
				if err := setReuseAddr(fd); err != nil {
					controlErr = fmt.Errorf("set SO_REUSEADDR failed: %w", err)
					return
				}
			}

			// 设置 SO_REUSEPORT（仅 Linux 支持）
			if lc.ReusePort {
				if err := setReusePort(fd); err != nil {
					controlErr = fmt.Errorf("set SO_REUSEPORT failed: %w", err)
					return
				}
			}
		})
		if err != nil {
			return err
		}
		return controlErr
	}
}

// Listen 使用配置的选项创建 Listener
func (lc *ListenConfig) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	config := net.ListenConfig{
		Control: lc.Control(),
	}
	return config.Listen(ctx, network, address)
}

// ListenPacket 使用配置的选项创建 PacketConn（UDP）
func (lc *ListenConfig) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	config := net.ListenConfig{
		Control: lc.Control(),
	}
	return config.ListenPacket(ctx, network, address)
}

// SetReuseAddr 配置 SO_REUSEADDR 和 SO_REUSEPORT
func SetReuseAddr(conn net.Conn) error {
	rawConn, err := getRawConnFromConn(conn)
	if err != nil {
		return err
	}

	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		controlErr = setReuseAddr(fd)
	}); err != nil {
		return fmt.Errorf("control raw conn failed: %w", err)
	}
	return controlErr
}

// SetReusePort 配置
func SetReusePort(conn net.Conn) error {
	rawConn, err := getRawConnFromConn(conn)
	if err != nil {
		return err
	}

	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		controlErr = setReusePort(fd)
	}); err != nil {
		return fmt.Errorf("control raw conn failed: %w", err)
	}
	return controlErr
}

// SetTCPNoDelay 禁用/启用 Nagle 算法（仅TCP）
func SetTCPNoDelay(conn net.Conn, enable bool) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return errors.New("conn is not *net.TCPConn (TCP_NODELAY only for TCP)")
	}
	return tcpConn.SetNoDelay(enable)
}

// SetTCPKeepAlive 配置 TCP 保活
// enable: 是否启用 keepalive
// period: keepalive 探测间隔（跨平台支持）
func SetTCPKeepAlive(conn net.Conn, enable bool, period time.Duration) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return errors.New("conn is not *net.TCPConn (keepalive only for TCP)")
	}

	// 启用/禁用 keepalive
	if err := tcpConn.SetKeepAlive(enable); err != nil {
		return fmt.Errorf("set keepalive enable failed: %w", err)
	}

	if !enable {
		return nil
	}

	// 设置 keepalive 间隔（如果提供了 period）
	if period > 0 {
		if err := tcpConn.SetKeepAlivePeriod(period); err != nil {
			return fmt.Errorf("set keepalive period failed: %w", err)
		}
	}

	return nil
}

// SetRcvBuffer 配置接收/发送缓冲区（TCP/UDP 通用）
// isTCP: true=TCP Conn，false=UDP Conn
// rcvBuf: 接收缓冲区大小（字节）
// sndBuf: 发送缓冲区大小（字节）
func SetRcvBuffer(conn net.Conn, rcvBuf int) error {
	rawConn, err := getRawConnFromConn(conn)
	if err != nil {
		return err
	}

	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		// 设置接收缓冲区
		if err := setSockOptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, rcvBuf); err != nil {
			controlErr = fmt.Errorf("set SO_RCVBUF failed: %w", err)
			return
		}
	}); err != nil {
		return fmt.Errorf("control raw conn failed: %w", err)
	}

	return controlErr
}

func SetSndBuffer(conn net.Conn, sndBuf int) error {
	rawConn, err := getRawConnFromConn(conn)
	if err != nil {
		return err
	}
	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		// 设置发送缓冲区
		if err := setSockOptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, sndBuf); err != nil {
			controlErr = fmt.Errorf("set SO_SNDBUF failed: %w", err)
			return
		}
	}); err != nil {
		return fmt.Errorf("control raw conn failed: %w", err)
	}

	return controlErr
}

// SetTCPLinger 配置 SO_LINGER（优雅关闭，仅TCP）
// enable: 是否启用
// lingerSec: 启用时，关闭连接前等待的秒数
func SetTCPLinger(conn net.Conn, enable bool, lingerSec int) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return errors.New("conn is not *net.TCPConn (linger only for TCP)")
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return fmt.Errorf("get TCP conn raw conn failed: %w", err)
	}

	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		linger := syscall.Linger{
			Onoff:  0,
			Linger: 0,
		}
		if enable {
			linger.Onoff = 1
			linger.Linger = int32(lingerSec)
		}

		if err := setSockOptLinger(fd, syscall.SOL_SOCKET, syscall.SO_LINGER, &linger); err != nil {
			controlErr = fmt.Errorf("set SO_LINGER failed: %w", err)
			return
		}
	}); err != nil {
		return fmt.Errorf("control raw conn failed: %w", err)
	}

	return controlErr
}

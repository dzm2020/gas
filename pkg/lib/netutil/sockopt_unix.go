//go:build !windows

package netutil

import "syscall"

// setSockOptInt Unix 平台设置 socket 整数选项
func setSockOptInt(fd uintptr, level, opt, value int) error {
	return syscall.SetsockoptInt(int(fd), level, opt, value)
}

// setSockOptLinger Unix 平台设置 socket Linger 选项
func setSockOptLinger(fd uintptr, level, opt int, linger *syscall.Linger) error {
	return syscall.SetsockoptLinger(int(fd), level, opt, linger)
}

// setReusePort 设置 SO_REUSEPORT 选项（仅 Linux 支持）
func setReusePort(fd uintptr) error {
	// SO_REUSEPORT 仅在 Linux 上定义，值为 15
	const SO_REUSEPORT = 15
	return setSockOptInt(fd, syscall.SOL_SOCKET, SO_REUSEPORT, 1)
}


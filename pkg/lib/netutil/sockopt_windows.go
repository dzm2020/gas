//go:build windows

package netutil

import "syscall"

// setSockOptInt Windows 平台设置 socket 整数选项
func setSockOptInt(fd uintptr, level, opt, value int) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), level, opt, value)
}

// setSockOptLinger Windows 平台设置 socket Linger 选项
func setSockOptLinger(fd uintptr, level, opt int, linger *syscall.Linger) error {
	return syscall.SetsockoptLinger(syscall.Handle(fd), level, opt, linger)
}

// setSockOptInt Windows 平台不支持
func setReusePort(fd uintptr) error {
	return nil
}

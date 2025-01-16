//go:build windows
// +build windows

package broadcastudp

import (
	"syscall"
)

func Upgrade(fd uintptr) error {
	// 设置 socket 选项为广播模式 (SO_BROADCAST) 不同的平台系统方法不同
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
}

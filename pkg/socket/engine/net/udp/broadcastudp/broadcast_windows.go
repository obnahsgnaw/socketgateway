//go:build windows
// +build windows

package broadcastudp

import (
	"net"
	"syscall"
)

func Upgrade(conn *net.UDPConn) error {
	// 获取文件描述符
	f, err := conn.File()
	if err != nil {
		return err
	}
	fileDescriptor := f.Fd()

	// 设置 socket 选项为广播模式 (SO_BROADCAST) 不同的平台系统方法不同
	err = syscall.SetsockoptInt(syscall.Handle(fileDescriptor), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	return err
}

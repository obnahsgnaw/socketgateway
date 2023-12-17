package sockettype

import "github.com/obnahsgnaw/application/servertype"

// SocketType 服务类型
type SocketType string

func (s SocketType) String() string {
	return string(s)
}

// 服务类型定义
const (
	TCP  SocketType = "tcp"
	TCP4 SocketType = "tcp4"
	TCP6 SocketType = "tcp6"
	UDP  SocketType = "udp"
	UDP4 SocketType = "udp4"
	UDP6 SocketType = "udp6"
	WSS  SocketType = "wss"
)

func (s SocketType) IsTcp() bool {
	return s == TCP || s == TCP4 || s == TCP6
}

func (s SocketType) IsUdp() bool {
	return s == UDP || s == UDP4 || s == UDP6
}

func (s SocketType) IsWss() bool {
	return s == WSS
}

func (s SocketType) ToServerType() (sst servertype.ServerType) {
	switch s {
	case TCP, TCP4, TCP6:
		sst = servertype.Tcp
		break
	case WSS:
		sst = servertype.Wss
		break
	case UDP, UDP4, UDP6:
		sst = servertype.Udp
		break
	default:
		panic("trans socket type to server type failed")
	}
	return
}

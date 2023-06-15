package sockettype

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

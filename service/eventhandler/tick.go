package eventhandler

import (
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"time"
)

type TickHandler func(*socket.Server, socket.Conn) bool

// 连接x秒后未认证 踢掉
func authTicker(ttl time.Duration) TickHandler {
	return func(server *socket.Server, conn socket.Conn) bool {
		if !conn.Context().Authed() && conn.Context().ConnectedAt().Add(ttl).Before(time.Now()) {
			conn.Context().SetOptional("close_reason", "close by auth checker")
			conn.Close()
		}
		return true
	}
}

// 心跳检查时间内未发送数据 踢掉
func heartbeatTicker(ttl time.Duration) TickHandler {
	return func(server *socket.Server, conn socket.Conn) bool {
		if conn.Context().LastActiveAt().Add(ttl).Before(time.Now()) {
			conn.Context().SetOptional("close_reason", "close by heartbeat checker")
			conn.Close()
		}
		return true
	}
}

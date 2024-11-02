package eventhandler

import (
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler/connutil"
	"time"
)

type TickHandler func(*socket.Server, socket.Conn) bool

// 连接x秒后未认证 踢掉
func authTicker(ttl time.Duration) TickHandler {
	return func(server *socket.Server, conn socket.Conn) bool {
		if !conn.Context().Authed() && conn.Context().ConnectedAt().Add(ttl).Before(time.Now()) {
			connutil.SetCloseReason(conn, "close by auth checker")
			conn.Close()
		}
		return true
	}
}

// 心跳检查时间内未发送数据 踢掉
func heartbeatTicker(ttl time.Duration) TickHandler {
	return func(server *socket.Server, conn socket.Conn) bool {
		v := connutil.GetHeartbeatInterval(conn)
		if v > 0 {
			ttl = time.Duration(v) * time.Second
		}
		if conn.Context().LastActiveAt().Add(ttl).Before(time.Now()) {
			connutil.SetCloseReason(conn, "close by heartbeat checker")
			conn.Close()
		}
		return true
	}
}

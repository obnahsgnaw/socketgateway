package udp

import "github.com/obnahsgnaw/socketgateway/pkg/socket"

type Option func(*Server)

func FdProvider(f func() int64) Option {
	return func(s *Server) {
		if f != nil {
			s.fdProvider = f
		}
	}
}

func Open(f func(conn socket.Conn)) Option {
	return func(s *Server) {
		if f != nil {
			s.onConnect = f
		}
	}
}

func Close(f func(conn socket.Conn, err error)) Option {
	return func(s *Server) {
		if f != nil {
			s.onDisconnect = f
		}
	}
}

func Message(f func(conn socket.Conn)) Option {
	return func(s *Server) {
		if f != nil {
			s.onMessage = f
		}
	}
}

func Network(network string) Option {
	return func(s *Server) {
		if network == "udp" || network == "udp4" || network == "udp6" {
			s.network = network
		}
	}
}

func BroadcastListen() Option {
	return func(s *Server) {
		s.broadcastListen = true
	}
}

func BroadcastSend(addr string) Option {
	return func(s *Server) {
		s.broadcastSendAddr = addr
	}
}

func IdentifyProvider(fn func([]byte) string) Option {
	return func(s *Server) {
		if fn != nil {
			s.identifyProvider = fn
		}
	}
}

func BodyMax(max int) Option {
	return func(s *Server) {
		s.bodyMax = max
	}
}

package tcp

import "github.com/obnahsgnaw/socketgateway/pkg/socket"

type Option func(*Server)

func Network(network string) Option {
	return func(e *Server) {
		if network == "tcp" {
			e.network = network
		}
	}
}

func FdProvider(f func() int64) Option {
	return func(s *Server) {
		if f != nil {
			s.fdProvider = f
		}
	}
}

func Connect(f func(conn socket.Conn)) Option {
	return func(s *Server) {
		if f != nil {
			s.onConnect = f
		}
	}
}

func Disconnect(f func(conn socket.Conn, err error)) Option {
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

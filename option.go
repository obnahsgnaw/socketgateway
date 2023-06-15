package socketgateway

import (
	"github.com/obnahsgnaw/application/pkg/security"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	"time"
)

type Option func(s *Server)

func RegEnable() Option {
	return func(s *Server) {
		s.regEnable = true
	}
}

func Keepalive(interval uint) Option {
	return func(s *Server) {
		s.keepalive = interval
	}
}

func Poll() Option {
	return func(s *Server) {
		s.poll = true
	}
}

func AuthCheck(interval time.Duration) Option {
	return func(s *Server) {
		s.e.With(eventhandler.AuthCheck(interval))
	}
}

func Heartbeat(interval time.Duration) Option {
	return func(s *Server) {
		s.e.With(eventhandler.Heartbeat(interval))
	}
}

func Auth() Option {
	return func(s *Server) {
		s.e.With(eventhandler.Auth())
	}
}

func Tick(interval time.Duration) Option {
	return func(s *Server) {
		s.tickInterval = interval
		s.e.With(eventhandler.Tick(interval))
	}
}

func Crypto(crypto *security.EsCrypto) Option {
	return func(s *Server) {
		s.e.With(eventhandler.Crypto(crypto))
	}
}

func StaticCryptKey(key []byte) Option {
	return func(s *Server) {
		s.e.With(eventhandler.StaticCryptKey(key))
	}
}

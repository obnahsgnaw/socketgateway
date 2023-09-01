package socketgateway

import (
	"github.com/obnahsgnaw/socketgateway/service/codec"
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
func ReuseAddr() Option {
	return func(s *Server) {
		s.reuseAddr = true
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

func Auth(address string) Option {
	return func(s *Server) {
		s.authAddress = address
		s.e.With(eventhandler.Auth())
	}
}

func Tick(interval time.Duration) Option {
	return func(s *Server) {
		s.tickInterval = interval
		s.e.With(eventhandler.Tick(interval))
	}
}

func Crypto(crypto eventhandler.Cryptor, noAuthKey []byte) Option {
	return func(s *Server) {
		s.crypto = crypto
		s.noAuthStaticKey = noAuthKey
		s.e.With(eventhandler.Crypto(crypto, noAuthKey))
	}
}

func CodecProvider(p codec.Provider) Option {
	return func(s *Server) {
		s.e.With(eventhandler.CodecProvider(p))
	}
}

func CodedProvider(p codec.DataBuilderProvider) Option {
	return func(s *Server) {
		s.e.With(eventhandler.CodedProvider(p))
	}
}

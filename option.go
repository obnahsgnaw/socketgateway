package socketgateway

import (
	"github.com/gin-gonic/gin"
	rpc2 "github.com/obnahsgnaw/rpc"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	"github.com/obnahsgnaw/socketutil/codec"
	"time"
)

type Option func(s *Server)

func RegEnable() Option {
	return func(s *Server) {
		s.regEnable = true
	}
}

func RouteDebug() Option {
	return func(s *Server) {
		s.routeDebug = true
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
		s.regEo(eventhandler.AuthCheck(interval))
	}
}

func Heartbeat(interval time.Duration) Option {
	return func(s *Server) {
		s.regEo(eventhandler.Heartbeat(interval))
	}
}

func Auth(address string) Option {
	return func(s *Server) {
		s.authAddress = address
		s.regEo(eventhandler.Auth())
	}
}

func Tick(interval time.Duration) Option {
	return func(s *Server) {
		s.tickInterval = interval
		s.regEo(eventhandler.Tick(interval))
	}
}

func Crypto(crypto eventhandler.Cryptor, noAuthKey []byte) Option {
	return func(s *Server) {
		s.crypto = crypto
		s.noAuthStaticKey = noAuthKey
		s.regEo(eventhandler.Crypto(crypto, noAuthKey))
	}
}

func CodecProvider(p codec.Provider) Option {
	return func(s *Server) {
		s.regEo(eventhandler.CodecProvider(p))
	}
}

func CodedProvider(p codec.DataBuilderProvider) Option {
	return func(s *Server) {
		s.regEo(eventhandler.CodedProvider(p))
	}
}

func RpcServerIns(ins *rpc2.Server) Option {
	return func(s *Server) {
		s.withRpcServerIns(ins)
	}
}

func RpcServer(port int) Option {
	return func(s *Server) {
		s.withRpcServer(port)
	}
}

func DocServerIns(e *gin.Engine, ePort int, docProxyPrefix string) Option {
	return func(s *Server) {
		s.withDocServerIns(e, ePort, docProxyPrefix)
	}
}

func DocServ(port int, docProxyPrefix string, projPrefixed bool) Option {
	return func(s *Server) {
		s.withDocServer(port, docProxyPrefix, projPrefixed)
	}
}

func Watcher(watcher eventhandler.LogWatcher) Option {
	return func(s *Server) {
		s.watchLog(watcher)
	}
}

func Ticker(name string, ticker eventhandler.TickHandler) Option {
	return func(s *Server) {
		s.addTicker(name, ticker)
	}
}

func Engine(e socket.Engine) Option {
	return func(s *Server) {
		s.setSocketEngine(e)
	}
}

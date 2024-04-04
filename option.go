package socketgateway

import (
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/http"
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
		s.addEventOption(eventhandler.AuthCheck(interval))
	}
}

func Heartbeat(interval time.Duration) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.Heartbeat(interval))
	}
}

func Auth(p AuthProvider) Option {
	return func(s *Server) {
		s.authProvider = p
		s.addEventOption(eventhandler.Auth())
	}
}

func DefaultUser(u *socket.AuthUser) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.DefaultUser(u))
	}
}

func Tick(interval time.Duration) Option {
	return func(s *Server) {
		s.tickInterval = interval
		s.addEventOption(eventhandler.Tick(interval))
	}
}

func Crypto(crypto eventhandler.Cryptor, noAuthKey []byte) Option {
	return func(s *Server) {
		s.cryptor = crypto
		s.noAuthStaticKey = noAuthKey
		s.addEventOption(eventhandler.Crypto(crypto, noAuthKey))
	}
}

func CodecProvider(p codec.Provider) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.CodecProvider(p))
	}
}

func CodedProvider(p codec.DataBuilderProvider) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.CodedProvider(p))
	}
}

func Rpc(ins *rpc2.Server) Option {
	return func(s *Server) {
		s.rpcServer = ins
		s.rpcServer.AddRegInfo(s.socketType.String()+"-gateway", utils.ToStr(s.socketType.String(), "-", s.id, "-rpc"), rpc2.NewPServer(s.id, s.serverType))
	}
}

func Doc(e *http.Http) Option {
	return func(s *Server) {
		s.docServer = newDocServerWithEngine(e, s.app.Cluster().Id(), s.docConfig())
	}
}

func Watcher(watcher eventhandler.LogWatcher) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.Watcher(watcher))
	}
}

func Ticker(name string, ticker eventhandler.TickHandler) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.Ticker(name, ticker))
	}
}

func Engine(e socket.Engine) Option {
	return func(s *Server) {
		s.engine = e
	}
}

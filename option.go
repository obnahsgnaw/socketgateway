package socketgateway

import (
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/application/servertype"
	"github.com/obnahsgnaw/goutils/security/coder"
	"github.com/obnahsgnaw/goutils/security/esutil"
	"github.com/obnahsgnaw/http"
	rpc2 "github.com/obnahsgnaw/rpc"
	"github.com/obnahsgnaw/socketgateway/pkg/mqtt"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	mqtt2 "github.com/obnahsgnaw/socketgateway/pkg/socket/engine/custom/mqtt"
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
		s.addEventOption(eventhandler.Auth(p))
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

func Crypto(esTp esutil.EsType, esMode esutil.EsMode) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.Crypto(esTp, esMode))
	}
}

func SecPrivateKey(key []byte) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.SecPrivateKey(key))
	}
}

func SecEncoder(encoder coder.Encoder) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.SecEncoder(encoder))
	}
}

func SecEncode(encode bool) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.SecEncode(encode))
	}
}

func SecTtl(ttl int64) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.SecTtl(ttl))
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
		s.rpcServer.AddRegInfo(s.id, utils.ToStr(s.rawSocketType.String(), "-", s.id, "-rpc"), rpc2.NewPServer(s.id, servertype.ServerType("socket-gw@"+s.businessChannel)))
	}
}

func RpcIns(ins *rpc2.Server) Option {
	return func(s *Server) {
		s.rpcServer = ins
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

func Interceptor(i func() error) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.Interceptor(i))
	}
}

func DefaultDataType(name codec.Name) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.DefaultDataType(name))
	}
}

func ProtocolCoder(coder codec.Codec) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.ProtocolCoder(coder))
	}
}

func PackageCoder(coder eventhandler.PackageBuilder) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.PackageCoder(coder))
	}
}

func DataCoder(coder codec.DataBuilder) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.DataCoder(coder))
	}
}

func Engine(e socket.Engine) Option {
	return func(s *Server) {
		s.engine = e
	}
}

func UserAuthenticate() Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.UserAuthenticate())
	}
}

func PrivateKeyForAll() Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.PrivateKeyForAll())
	}
}

func AuthenticatedCallback(fn ...func(socket.Conn)) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.AuthenticatedCallback(fn...))
	}
}

func AuthenticatedBefores(fn ...func(socket.Conn, []byte) []byte) Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.AuthenticatedBefores(fn...))
	}
}

func WithoutDftUserAuthenticate() Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.WithoutDftUserAuthenticate())
	}
}

func WithDftUserAuthenticate() Option {
	return func(s *Server) {
		s.addEventOption(eventhandler.WithDftUserAuthenticate())
	}
}

func Name(subId, name string) Option {
	return func(s *Server) {
		if subId != "" && name != "" {
			s.id = "gateway-" + subId
			s.name = name
		}
	}
}

func Mqtt(tcpAddr string, option ...mqtt.Option) Option {
	return func(s *Server) {
		s.mqttAddr = tcpAddr
		s.mqttOptions = option
	}
}

func MqttRawTopic(topics ...mqtt2.QosTopic) Option {
	return func(s *Server) {
		s.mqttRawTopics = append(s.mqttRawTopics, topics...)
	}
}

func MqttTopic(clientTopic, serverTopic mqtt2.QosTopic) Option {
	return func(s *Server) {
		s.mqttClientTopic = &clientTopic
		s.mqttServerTopic = &serverTopic
	}
}

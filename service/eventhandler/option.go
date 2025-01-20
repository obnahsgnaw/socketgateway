package eventhandler

import (
	"github.com/obnahsgnaw/goutils/security/coder"
	"github.com/obnahsgnaw/goutils/security/esutil"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"time"
)

type Option func(event *Event)

func AuthCheck(interval time.Duration) Option {
	return func(event *Event) {
		if interval > 0 {
			event.AddTicker("auth-ticker", authTicker(interval))
		}
	}
}

func Heartbeat(interval time.Duration) Option {
	return func(event *Event) {
		if interval > 0 {
			event.AddTicker("heartbeat-ticker", heartbeatTicker(interval))
		}
	}
}

func Auth(authProvider AuthProvider) Option {
	return func(event *Event) {
		event.authEnable = true
		event.authProvider = authProvider
		event.AddTicker("auth-ticker", authTicker(time.Second*10))
	}
}

func DefaultUser(u *socket.AuthUser) Option {
	return func(event *Event) {
		if u.Attr == nil {
			u.Attr = make(map[string]string)
		}
		event.defaultUser = u
	}
}

func Tick(interval time.Duration) Option {
	return func(event *Event) {
		event.tickInterval = interval
	}
}

func SecPrivateKey(key []byte) Option {
	return func(event *Event) {
		event.commonPrivateKey = key
	}
}

func SecEncoder(encoder coder.Encoder) Option {
	return func(event *Event) {
		event.secEncoder = encoder
		event.es = esutil.New(event.esTp, event.esMode, esutil.Encoder(encoder))
	}
}

func SecEncode(encode bool) Option {
	return func(event *Event) {
		event.secEncode = encode
	}
}

func SecTtl(ttl int64) Option {
	return func(event *Event) {
		event.secTtl = ttl
	}
}

func Crypto(esTp esutil.EsType, esMode esutil.EsMode) Option {
	return func(event *Event) {
		event.esTp = esTp
		event.esMode = esMode
		event.es = esutil.New(esTp, esMode, esutil.Encoder(event.secEncoder))
	}
}

func Logger(l *zap.Logger) Option {
	return func(event *Event) {
		event.logger = l
	}
}

func CodecProvider(p codec.Provider) Option {
	return func(event *Event) {
		event.codecProvider = p
	}
}

func CodedProvider(p codec.DataBuilderProvider) Option {
	return func(event *Event) {
		event.codedProvider = p
	}
}

func Watcher(watcher LogWatcher) Option {
	return func(event *Event) {
		event.WatchLog(watcher)
	}
}

func Ticker(name string, ticker TickHandler) Option {
	return func(event *Event) {
		event.AddTicker(name, ticker)
	}
}

func Interceptor(i func() error) Option {
	return func(event *Event) {
		event.openInterceptor = i
	}
}

func DefaultDataType(name codec.Name) Option {
	return func(event *Event) {
		event.defDataType = name
	}
}

func ProtocolCoder(coder codec.Codec) Option {
	return func(event *Event) {
		event.protocolCoder = coder
	}
}

func PackageCoder(coder PackageBuilder) Option {
	return func(event *Event) {
		event.packageCoder = coder
	}
}

func DataCoder(coder codec.DataBuilder) Option {
	return func(event *Event) {
		event.dataCoder = coder
	}
}

func UserAuthenticate() Option {
	return func(event *Event) {
		event.withUserAuthenticate()
	}
}

func PrivateKeyForAll() Option {
	return func(event *Event) {
		event.commonCertForAll = true
	}
}

func AuthenticatedCallback(fn ...func(socket.Conn)) Option {
	return func(event *Event) {
		event.authenticatedCallbacks = append(event.authenticatedCallbacks, fn...)
	}
}

package eventhandler

import (
	"github.com/obnahsgnaw/application/pkg/security"
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

func Auth() Option {
	return func(event *Event) {
		event.authEnable = true
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
		event.privateKey = key
	}
}

func SecEncoder(encoder security.Encoder) Option {
	return func(event *Event) {
		event.secEncoder = encoder
		event.rsa.SetEncoder(encoder)
		event.es.SetEncoder(encoder)
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

func Crypto(esTp security.EsType, esMode security.EsMode) Option {
	return func(event *Event) {
		event.es = security.NewEsCrypto(esTp, esMode)
		if event.secEncoder != nil {
			event.es.SetEncoder(event.secEncoder)
		}
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
		event.interceptor = i
	}
}

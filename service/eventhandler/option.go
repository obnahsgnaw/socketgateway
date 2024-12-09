package eventhandler

import (
	"crypto"
	"github.com/obnahsgnaw/goutils/security/coder"
	"github.com/obnahsgnaw/goutils/security/esutil"
	"github.com/obnahsgnaw/goutils/security/rsautil"
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

func SecEncoder(encoder coder.Encoder) Option {
	return func(event *Event) {
		event.secEncoder = encoder
		event.rsa = rsautil.New(rsautil.PKCS1Public(), rsautil.PKCS1Private(), rsautil.SignHash(crypto.SHA256), rsautil.Encoder(encoder))
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
		event.interceptor = i
	}
}

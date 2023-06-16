package eventhandler

import (
	"github.com/obnahsgnaw/application/pkg/security"
	"github.com/obnahsgnaw/socketgateway/service/codec"
	"go.uber.org/zap"
	"time"
)

type Option func(event *Event)

func AuthCheck(interval time.Duration) Option {
	return func(event *Event) {
		if interval > 0 {
			event.AddTicker(authTicker(interval))
		}
	}
}

func Heartbeat(interval time.Duration) Option {
	return func(event *Event) {
		if interval > 0 {
			event.AddTicker(heartbeatTicker(interval))
		}
	}
}

func Auth() Option {
	return func(event *Event) {
		event.authEnable = true
	}
}

func Tick(interval time.Duration) Option {
	return func(event *Event) {
		event.tickInterval = interval
	}
}

func Crypto(crypto *security.EsCrypto) Option {
	return func(event *Event) {
		event.crypto = crypto
	}
}

func StaticCryptKey(key []byte) Option {
	return func(event *Event) {
		event.staticEsKey = key
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

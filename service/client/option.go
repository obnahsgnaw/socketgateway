package client

import (
	client2 "github.com/obnahsgnaw/socketgateway/pkg/client"
	"go.uber.org/zap"
	"time"
)

type Option func(*Client)

func Retry(interval time.Duration) Option {
	return func(client *Client) {
		client.c.With(client2.Retry(interval))
	}
}

func Timeout(timeout time.Duration) Option {
	return func(client *Client) {
		client.c.With(client2.Timeout(timeout))
	}
}

func Keepalive(interval time.Duration) Option {
	return func(client *Client) {
		client.c.With(client2.Keepalive(interval))
	}
}

func Connect(handler func(index int)) Option {
	return func(client *Client) {
		client.c.With(client2.Connect(handler))
	}
}

func Disconnect(handler func(index int)) Option {
	return func(client *Client) {
		client.c.With(client2.Disconnect(handler))
	}
}

func Logger(l *zap.Logger) Option {
	return func(client *Client) {
		client.logger = l
		client.c.With(client2.Logger(l))
	}
}

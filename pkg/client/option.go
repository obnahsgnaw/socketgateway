package client

import (
	"go.uber.org/zap"
	"time"
)

type Option func(*Client)

func Retry(interval time.Duration) Option {
	return func(client *Client) {
		client.retryInterval = interval
	}
}

func Timeout(timeout time.Duration) Option {
	return func(client *Client) {
		client.connectTimeout = timeout
	}
}

func Keepalive(interval time.Duration) Option {
	return func(client *Client) {
		client.keepAlive = interval
	}
}

func Logger(logger *zap.Logger) Option {
	return func(client *Client) {
		client.logger = logger
	}
}

func Connect(handler func(index int)) Option {
	return func(client *Client) {
		client.connectedHandler = handler
	}
}

func Disconnect(handler func(index int)) Option {
	return func(client *Client) {
		client.disconnectedHandler = handler
	}
}

func Message(handler func(pkg []byte)) Option {
	return func(client *Client) {
		client.messageHandler = handler
	}
}

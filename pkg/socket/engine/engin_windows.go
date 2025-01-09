//go:build windows
// +build windows

package engine

import (
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/net"
)

func Default() socket.Engine {
	return net.New()
}

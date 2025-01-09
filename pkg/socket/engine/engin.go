//go:build !windows
// +build !windows

package engine

import (
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/gnet"
)

func Default() socket.Engine {
	return gnet.New()
}

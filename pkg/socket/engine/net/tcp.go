package net

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/net/tcp"
	"strings"
	"sync/atomic"
)

type tcpEngineHandler struct {
	e *Engine
	s *tcp.Server
}

func newTcpEngineHandler(e *Engine, addr string) *tcpEngineHandler {
	network, address := parseProtoAddr(addr)
	network = strings.ToLower(network)
	tp := tcp.New(address,
		tcp.Network(network),
		tcp.FdProvider(func() int64 {
			atomic.AddInt64(&e.index, 1)
			return atomic.LoadInt64(&e.index)
		}),
		tcp.Connect(func(c socket.Conn) {
			e.connections.Store(c.Fd(), c)
			e.event.OnOpen(e.server, c)
		}),
		tcp.Disconnect(func(c socket.Conn, err error) {
			e.event.OnClose(e.server, c, errors.New("close by peer"))
			e.connections.Delete(c.Fd())
		}),
		tcp.Message(func(c socket.Conn) {
			e.event.OnTraffic(e.server, c)
		}),
	)
	return &tcpEngineHandler{e: e, s: tp}
}

func (h *tcpEngineHandler) Init() error {
	return h.s.Init()
}

func (h *tcpEngineHandler) Run(ctx context.Context) {
	h.s.Run(ctx)
}

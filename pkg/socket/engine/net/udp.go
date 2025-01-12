package net

import (
	"context"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/net/udp"
	"sync/atomic"
)

type udpEngineHandler struct {
	e *Engine
	u *udp.Server
}

func newUdpEngineHandler(e *Engine, network string, port int) *udpEngineHandler {
	u := udp.New(port,
		udp.FdProvider(func() int64 {
			atomic.AddInt64(&e.index, 1)
			return atomic.LoadInt64(&e.index)
		}),
		udp.Open(func(c socket.Conn) {
			e.event.OnOpen(e.server, c)
		}),
		udp.Close(func(c socket.Conn, err error) {
			e.event.OnClose(e.server, c, err)
		}),
		udp.Message(func(c socket.Conn) {
			e.event.OnTraffic(e.server, c)
		}),
		udp.Network(network),
	)
	return &udpEngineHandler{
		e: e,
		u: u,
	}
}

func (h *udpEngineHandler) BroadcastMode(sendAddr string) {
	h.u.With(udp.BroadcastListen())
	h.u.With(udp.BroadcastSend(sendAddr))
}

func (h *udpEngineHandler) IdentifyProvider(fn func([]byte) string) {
	h.u.With(udp.IdentifyProvider(fn))
}

func (h *udpEngineHandler) BodyMax(size int) {
	h.u.With(udp.BodyMax(size))
}

func (h *udpEngineHandler) Init() error {
	return h.u.Init()
}

func (h *udpEngineHandler) Run(ctx context.Context) {
	h.u.Run(ctx)
}

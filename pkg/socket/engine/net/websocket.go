package net

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	websocket2 "github.com/obnahsgnaw/socketgateway/pkg/socket/engine/net/websocket"
	"sync/atomic"
)

type wssEngineHandler struct {
	e *Engine
	s *websocket2.Server
}

func newWssEngineHandler(e *Engine, port int) *wssEngineHandler {
	ws := websocket2.New(port,
		websocket2.FdProvider(func() int64 {
			atomic.AddInt64(&e.index, 1)
			return atomic.LoadInt64(&e.index)
		}),
		websocket2.Connect(func(c socket.Conn) {
			e.connections.Store(c.Fd(), c)
			e.event.OnOpen(e.server, c)
		}),
		websocket2.Disconnect(func(c socket.Conn, err error) {
			e.event.OnClose(e.server, c, errors.New("close by peer"))
			e.connections.Delete(c.Fd())
		}),
		websocket2.Message(func(c socket.Conn) {
			e.event.OnTraffic(e.server, c)
		}),
	)
	return &wssEngineHandler{
		e: e,
		s: ws,
	}
}

func (h *wssEngineHandler) Init() error {
	return h.s.Init()
}

func (h *wssEngineHandler) Run(ctx context.Context) {
	h.s.Run(ctx)
}

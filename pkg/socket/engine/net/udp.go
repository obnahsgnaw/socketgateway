package net

import (
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
	"sync/atomic"
)

type udpEngineHandler struct {
	e       *Engine
	l       *net.UDPConn
	clients map[string]int64
	network string
	port    int
}

func newUdpEngineHandler(e *Engine, network string, port int) *udpEngineHandler {
	return &udpEngineHandler{
		e:       e,
		l:       nil,
		clients: make(map[string]int64),
		network: network,
		port:    port,
	}
}

func (h *udpEngineHandler) Init() error {
	l, err := net.ListenUDP(h.network, &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: h.port,
	})
	if err != nil {
		return err
	}
	h.l = l
	return nil
}

func (h *udpEngineHandler) Run() {
	for {
		var data [1024]byte
		n, addr, err := h.l.ReadFromUDP(data[:])
		if err == nil && n > 0 {
			var fd int64
			var ok bool
			if fd, ok = h.clients[addr.String()]; !ok {
				atomic.AddInt64(&h.e.index, 1)
				fd = atomic.LoadInt64(&h.e.index)
				h.clients[addr.String()] = fd
			}
			c := newUdpConn(int(fd), h.l, addr, socket.NewContext(), func(udpAddr *net.UDPAddr) {
				delete(h.clients, udpAddr.String())
			})
			c.pkg = append(c.pkg, data[:n])
			h.e.event.OnTraffic(h.e.server, c)
		}
		if h.e.stopped {
			break
		}
	}
}

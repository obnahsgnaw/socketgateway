package net

import (
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
	"strings"
	"sync/atomic"
)

type tcpEngineHandler struct {
	e    *Engine
	l    net.Listener
	addr string
}

func newTcpEngineHandler(e *Engine, addr string) *tcpEngineHandler {
	return &tcpEngineHandler{e: e, addr: addr}
}

func (h *tcpEngineHandler) Init() error {
	network, address := parseProtoAddr(h.addr)
	network = strings.ToLower(network)
	l, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	h.l = l
	return nil
}

func (h *tcpEngineHandler) Run() {
	for {
		conn, err := h.l.Accept()
		if err == nil {
			atomic.AddInt64(&h.e.index, 1)
			fd := atomic.LoadInt64(&h.e.index)
			c := newConn(int(fd), conn, socket.NewContext())
			h.e.connections.Store(fd, c)
			h.e.event.OnOpen(h.e.server, c)
			go h.handConn(c)
		}
		if h.e.stopped {
			break
		}
	}
}

func (h *tcpEngineHandler) handConn(c *Conn) {
	defer func() {
		c.Close()
		h.e.event.OnClose(h.e.server, c, errors.New("close by peer"))
		h.e.connections.Delete(c.fd)
	}()
	var buf [1024]byte
	for {
		n, err := c.raw.Read(buf[:])
		if err != nil {
			break
		}
		if n > 0 {
			c.pkg = append(c.pkg, buf[:n])
			h.e.event.OnTraffic(h.e.server, c)
		}
	}
}

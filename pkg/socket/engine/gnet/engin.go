package gnet

import (
	"context"
	"github.com/gobwas/ws"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/panjf2000/gnet/v2"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultTickDelay = time.Second * 60

// Engine 封装上层的事件处理 转发给eventHandler
type Engine struct {
	ctx         context.Context
	addr        string
	event       socket.Event
	server      *socket.Server
	connections sync.Map // fd=>conn
	ws          bool
}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) OnBoot(_ gnet.Engine) (action gnet.Action) {
	e.event.OnBoot(e.server)
	return
}

func (e *Engine) OnShutdown(gnet.Engine) {
	e.event.OnShutdown(e.server)
}

func (e *Engine) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	connCtx := socket.NewContext()
	c.SetContext(connCtx)
	c1 := newConn(c, connCtx)
	e.connections.Store(c.Fd(), c1)
	e.event.OnOpen(e.server, c1)
	return
}

func (e *Engine) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	c1, _ := e.connections.LoadAndDelete(c.Fd())
	e.event.OnClose(e.server, c1.(*Conn), err)
	return
}

func (e *Engine) OnTraffic(c gnet.Conn) (action gnet.Action) {
	var c2 *Conn

	if e.server.Type().IsUdp() {
		connCtx := socket.NewContext()
		c.SetContext(connCtx)
		c2 = newConn(c, connCtx)
	} else {
		c1, _ := e.connections.Load(c.Fd())
		c2 = c1.(*Conn)
	}
	c2.Context().Active()
	if e.ws && !c2.Context().Upgraded() {
		if _, err := ws.Upgrade(c); err != nil {
			return gnet.Close
		}
		c2.Context().Upgrade()
	} else {
		e.event.OnTraffic(e.server, c2)
	}

	return
}

func (e *Engine) OnTick() (delay time.Duration, action gnet.Action) {
	delay = e.event.OnTick(e.server)
	if delay == 0 {
		delay = defaultTickDelay
	}
	return
}

func (e *Engine) Run(ctx context.Context, s *socket.Server, ee socket.Event, t sockettype.SocketType, p int, c *socket.Config) (err error) {
	e.server = s
	e.event = ee
	e.ctx = ctx
	e.ws = t == sockettype.WSS
	if strings.Contains(t.String(), "udp") {
		e.addr = "udp://:" + strconv.Itoa(p)
	} else {
		e.addr = "tcp://:" + strconv.Itoa(p)
	}
	var options []gnet.Option
	options = append(options, gnet.WithMulticore(c.MultiCore))
	options = append(options, gnet.WithTicker(c.Ticker))
	if c.NoDelay {
		options = append(options, gnet.WithTCPNoDelay(gnet.TCPNoDelay))
	} else {
		options = append(options, gnet.WithTCPNoDelay(gnet.TCPDelay))
	}
	if c.Keepalive > 0 {
		options = append(options, gnet.WithTCPKeepAlive(time.Duration(c.Keepalive)*time.Second))
	}
	options = append(options, gnet.WithReuseAddr(c.ReuseAddr))
	return gnet.Run(e, e.addr, options...)
}

func (e *Engine) Stop() error {
	return gnet.Stop(e.ctx, e.addr)
}

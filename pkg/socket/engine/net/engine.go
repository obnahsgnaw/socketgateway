package net

import (
	"context"
	"errors"
	"fmt"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Engine 封装上层的事件处理 转发给eventHandler
type Engine struct {
	ctx         context.Context
	cancel      context.CancelFunc
	addr        string
	event       socket.Event
	server      *socket.Server
	index       int64
	connections sync.Map
	stopped     bool
	ws          bool
	t           sockettype.SocketType
}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Run(ctx context.Context, s *socket.Server, ee socket.Event, t sockettype.SocketType, p int, c *socket.Config) (err error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("%v\n", err)
		}
	}()
	e.server = s
	e.event = ee
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.ws = t == sockettype.WSS
	if e.ws {
		return errors.New("socket engine error: not support now")
	}
	e.addr = t.String() + "://:" + strconv.Itoa(p)
	network, addr := parseProtoAddr(e.addr)
	listener, err := net.Listen(network, addr)
	if err != nil {
		return err
	}
	e.event.OnBoot(e.server)

	if c.Ticker {
		go func(ctxx context.Context) {
			for {
				select {
				case <-ctxx.Done():
					break
				default:
					delay := e.event.OnTick(e.server)
					time.Sleep(delay)
				}
			}
		}(e.ctx)
	}

	for {
		conn, err := listener.Accept()
		if err == nil {
			atomic.AddInt64(&e.index, 1)
			fd := atomic.LoadInt64(&e.index)
			c := newConn(int(fd), conn, socket.NewContext())
			e.connections.Store(fd, c)
			e.event.OnOpen(e.server, c)
			go e.handConn(c)
		}
		if e.stopped {
			break
		}
	}

	e.event.OnShutdown(e.server)
	return nil
}

func (e *Engine) Stop() error {
	e.stopped = true
	return nil
}

func (e *Engine) handConn(c *Conn) {
	defer func() {
		c.Close()
		e.event.OnClose(e.server, c, errors.New("close by peer"))
		e.connections.Delete(c.fd)
	}()
	var buf [1024]byte
	for {
		n, err := c.raw.Read(buf[:])
		if err != nil {
			break
		}
		c.pkg = append(c.pkg, buf[:n])
		e.event.OnTraffic(e.server, c)
	}
}

package net

import (
	"context"
	"errors"
	"fmt"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Engine 封装上层的事件处理 转发给eventHandler
type Engine struct {
	ctx              context.Context
	cancel           context.CancelFunc
	addr             string
	event            socket.Event
	server           *socket.Server
	index            int64
	connections      sync.Map
	stopped          bool
	t                sockettype.SocketType
	udpBroadcast     bool
	udpBroadcastAddr string
	idProvider       func([]byte) string
}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Run(ctx context.Context, s *socket.Server, ee socket.Event, t sockettype.SocketType, port int, c *socket.Config) (err error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("%v\n", err)
		}
	}()
	e.server = s
	e.event = ee
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.addr = t.String() + "://:" + strconv.Itoa(port)
	var handler engineHandler

	if t.IsTcp() {
		handler = newTcpEngineHandler(e, e.addr)
	} else if t.IsUdp() {
		udpHdr := newUdpEngineHandler(e, t.String(), port)
		if e.udpBroadcast {
			udpHdr.BroadcastMode(e.udpBroadcastAddr)
		}
		if e.idProvider != nil {
			udpHdr.IdentifyProvider(e.idProvider)
		}
		handler = udpHdr
	} else if t.IsWss() {
		handler = newWssEngineHandler(e, port)
	} else {
		return errors.New("socket engine error: not support now")
	}
	if err = handler.Init(); err != nil {
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

	handler.Run(e.ctx)

	e.event.OnShutdown(e.server)
	return nil
}

func (e *Engine) Stop() error {
	e.stopped = true
	return nil
}

func (e *Engine) UdpBroadcast(sendAddr string) *Engine {
	e.udpBroadcast = true
	e.udpBroadcastAddr = sendAddr
	return e
}

func (e *Engine) UdpIdProvider(fn func([]byte) string) *Engine {
	e.idProvider = fn
	return e
}

func parseProtoAddr(addr string) (network, address string) {
	network = "tcp"
	address = strings.ToLower(addr)
	if strings.Contains(address, "://") {
		pair := strings.Split(address, "://")
		network = pair[0]
		address = pair[1]
	}
	return
}

type engineHandler interface {
	Init() error
	Run(context.Context)
}

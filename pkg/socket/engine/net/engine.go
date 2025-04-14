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
	ctx                context.Context
	cancel             context.CancelFunc
	addr               string
	event              socket.Event
	server             *socket.Server
	index              int64
	connections        sync.Map
	stopped            bool
	t                  sockettype.SocketType
	udpBroadcastListen bool
	udpBroadcastAddr   string
	udpIdProvider      func([]byte) string
	udpBodyMax         int
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
	if t == sockettype.HTTP || t == sockettype.MQTT {
		return errors.New("the engin not support the socket type: " + t.String())
	}
	e.server = s
	e.event = ee
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.addr = t.String() + "://:" + strconv.Itoa(port)
	var handler engineHandler

	if t.IsTcp() {
		handler = newTcpEngineHandler(e, e.addr)
	} else if t.IsUdp() {
		udpHdr := newUdpEngineHandler(e, t.String(), port)
		if e.udpBroadcastListen {
			udpHdr.BroadcastMode(e.udpBroadcastAddr)
		}
		if e.udpIdProvider != nil {
			udpHdr.IdentifyProvider(e.udpIdProvider)
		}
		if e.udpBodyMax > 0 {
			udpHdr.BodyMax(e.udpBodyMax)
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

func (e *Engine) UdpBroadcastListen() *Engine {
	e.udpBroadcastListen = true
	return e
}

func (e *Engine) UdpBroadcastSend(sendAddr string) *Engine {
	e.udpBroadcastAddr = sendAddr
	return e
}

func (e *Engine) UdpIdProvider(fn func([]byte) string) *Engine {
	e.udpIdProvider = fn
	return e
}

func (e *Engine) UdpBodyMax(size int) *Engine {
	e.udpBodyMax = size
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

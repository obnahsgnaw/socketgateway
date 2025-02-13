package eventhandler

import (
	"context"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/moc"
	"github.com/obnahsgnaw/socketutil/codec"
	"sync"
	"time"
)

type ProxyManager struct {
	connections sync.Map //map[string]*moc.Conn
}

var pm = &ProxyManager{}

func (m *ProxyManager) Sync(c *moc.Conn) *moc.Conn {
	v, ok := m.connections.Load(c.Id())
	if !ok {
		m.connections.Store(c.Id(), c)
		v = c
	}
	cc := v.(*moc.Conn)
	cc.Context().Active()
	return cc
}

func (m *ProxyManager) RangeConnections(f func(c socket.Conn) bool) {
	m.connections.Range(func(key, value interface{}) bool {
		return f(value.(*moc.Conn))
	})
}

func (m *ProxyManager) Remove(c *moc.Conn) {
	m.connections.Delete(c.Id())
}

func (e *Event) ProxyInit(c *moc.Conn, _ codec.Name, packedPkg []byte) (respPackage []byte, err error) {
	if err = e.openIntercept(); err != nil {
		return
	}
	pm.Remove(c)
	c = pm.Sync(c)
	e.ClearCoder(c)
	e.ClearCryptoKey(c)
	e.ss.Authenticate(c, nil)
	_, response, _, keyErr := e.authenticate(c, "", packedPkg)
	respPackage = []byte(response)
	if keyErr != nil {
		err = keyErr
	}
	return
}

// Proxy The proxy forwards other requests to handler (client -> Forwarding adapter -> Proxy)
func (e *Event) Proxy(c *moc.Conn, rqId string, packedPkg []byte) (rqAction, respAction codec.Action, rqData, respData, respPackage []byte, err error) {
	if err = e.openIntercept(); err != nil {
		return
	}
	c = pm.Sync(c)
	// Codec initialization
	if !e.CoderInitialized(c) || !e.CryptoKeyInitialized(c) {
		respPackage = []byte("222")
		return
	}
	// raw
	if e.handleRaw(c, rqId, packedPkg) {
		return
	}
	// Process message packets
	rqAction, respAction, rqData, respData, respPackage, err = e.handleMessage(c, rqId, packedPkg)
	if len(respPackage) > 0 {
		err1 := e.codecDecode(c, respPackage, func(pkg []byte) {
			respPackage = pkg
		})
		if err1 != nil && err == nil {
			err = err1
		}
	}
	return
}

func (e *Event) proxyOnTick() time.Duration {
	pm.RangeConnections(func(c socket.Conn) bool {
		for _, h := range e.tickHandlers {
			if !h(e.ss, c) {
				break
			}
		}
		return true
	})
	pm.RangeConnections(func(c socket.Conn) bool {
		cc := c.(*moc.Conn)
		if cc.Closed() {
			pm.Remove(cc)
		}
		return true
	})
	return e.tickInterval
}

func (e *Event) proxyTick(ctx context.Context) {
	if e.tickInterval > 0 {
		go func(ctx1 context.Context) {
			for {
				select {
				case <-ctx1.Done():
					return
				default:
					delay := e.proxyOnTick()
					if delay <= 0 {
						delay = 60 * time.Second
					}
					time.Sleep(delay)
				}
			}
		}(ctx)
	}
}

package websocket

import (
	"errors"
	"github.com/gorilla/websocket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
	"sync"
)

type Conn struct {
	fd          int
	connContext *socket.ConnContext
	raw         *websocket.Conn
	pkg         [][]byte
	closeCb     func(addr *net.UDPAddr)
	l           sync.Mutex
}

func newWssConn(fd int, c *websocket.Conn, ctx *socket.ConnContext) *Conn {
	return &Conn{
		fd:          fd,
		connContext: ctx,
		raw:         c,
	}
}

func (c *Conn) Fd() int {
	return c.fd
}

func (c *Conn) Context() *socket.ConnContext {
	return c.connContext
}

func (c *Conn) Read() ([]byte, error) {
	if len(c.pkg) == 0 {
		return nil, errors.New("conn error: no data to read")
	}
	b := c.pkg[0]
	c.pkg = c.pkg[1:]
	return b, nil
}

func (c *Conn) Write(b []byte) error {
	c.l.Lock()
	defer c.l.Unlock()
	return c.raw.WriteMessage(websocket.TextMessage, b)
}

func (c *Conn) Close() {
	if c.raw != nil {
		_ = c.raw.Close()
	}
}

func (c *Conn) LocalAddr() net.Addr {
	return c.raw.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.raw.RemoteAddr()
}

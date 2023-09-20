package net

import (
	"errors"
	"github.com/gorilla/websocket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
)

type WssConn struct {
	fd          int
	connContext *socket.ConnContext
	raw         *websocket.Conn
	pkg         [][]byte
	closeCb     func(addr *net.UDPAddr)
}

func newWssConn(fd int, c *websocket.Conn, ctx *socket.ConnContext) *WssConn {
	return &WssConn{
		fd:          fd,
		connContext: ctx,
		raw:         c,
	}
}

func (c *WssConn) Fd() int {
	return c.fd
}

func (c *WssConn) Context() *socket.ConnContext {
	return c.connContext
}

func (c *WssConn) Read() ([]byte, error) {
	if len(c.pkg) == 0 {
		return nil, errors.New("conn error: no data to read")
	}
	b := c.pkg[0]
	c.pkg = c.pkg[1:]
	return b, nil
}

func (c *WssConn) Write(b []byte) error {
	return c.raw.WriteMessage(websocket.TextMessage, b)
}

func (c *WssConn) Close() {
	_ = c.raw.Close()
}

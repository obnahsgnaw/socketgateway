package gnet

import (
	"errors"
	"github.com/gobwas/ws/wsutil"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/panjf2000/gnet/v2"
	"net"
)

// conn

type Conn struct {
	fd          int
	connContext *socket.ConnContext
	raw         gnet.Conn
	pkg         [][]byte
	err         error
}

func newConn(c gnet.Conn, ctx *socket.ConnContext) *Conn {
	return &Conn{
		fd:          c.Fd(),
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
	if c.connContext.Upgraded() {
		if c.err != nil {
			return nil, c.err
		}
		if len(c.pkg) == 0 {
			return nil, errors.New("conn error: no data to read")
		}
		b := c.pkg[0]
		c.pkg = c.pkg[1:]
		return b, nil
	}

	size := c.raw.InboundBuffered()
	buf := make([]byte, size)
	read, err := c.raw.Read(buf)
	if err != nil {
		return nil, err
	}
	if read < size {
		return nil, errors.New("read bytes len err")
	}
	return buf, nil
}

func (c *Conn) Write(b []byte) error {
	if c.connContext.Upgraded() {
		return wsutil.WriteServerText(c.raw, b)
	}
	_, err := c.raw.Write(b)
	return err
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

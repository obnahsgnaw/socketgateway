package gnet

import (
	"github.com/gobwas/ws/wsutil"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/panjf2000/gnet/v2"
)

// conn

type Conn struct {
	fd          int
	connContext *socket.ConnContext
	raw         gnet.Conn
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
		b, _, e := wsutil.ReadClientData(c.raw)
		return b, e
	}

	buf, err := c.raw.Peek(c.raw.InboundBuffered())
	if err == nil {
		if _, err = c.raw.Discard(len(buf)); err != nil {
			return nil, err
		}
	}
	return buf, err
}

func (c *Conn) Write(b []byte) error {
	if c.connContext.Upgraded() {
		return wsutil.WriteServerText(c.raw, b)
	}
	_, err := c.raw.Write(b)
	return err
}

func (c *Conn) Close() {
	_ = c.raw.Close()
}

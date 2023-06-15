package net

import (
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
	"strings"
)

// conn

type Conn struct {
	fd          int
	connContext *socket.ConnContext
	raw         net.Conn
	pkg         [][]byte
}

func newConn(fd int, c net.Conn, ctx *socket.ConnContext) *Conn {
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
		return nil, errors.New("no")
	}
	b := c.pkg[0]
	c.pkg = c.pkg[1:]
	return b, nil
}

func (c *Conn) Write(b []byte) error {
	_, err := c.raw.Write(b)
	return err
}

func (c *Conn) Close() {
	_ = c.raw.Close()
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

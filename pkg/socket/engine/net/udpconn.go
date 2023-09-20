package net

import (
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
)

type UdpConn struct {
	fd          int
	connContext *socket.ConnContext
	raw         *net.UDPConn
	addr        *net.UDPAddr
	pkg         [][]byte
	closeCb     func(addr *net.UDPAddr)
}

func newUdpConn(fd int, c *net.UDPConn, addr *net.UDPAddr, ctx *socket.ConnContext, closeCb func(udpAddr *net.UDPAddr)) *UdpConn {
	return &UdpConn{
		fd:          fd,
		connContext: ctx,
		raw:         c,
		addr:        addr,
		closeCb:     closeCb,
	}
}

func (c *UdpConn) Fd() int {
	return c.fd
}

func (c *UdpConn) Context() *socket.ConnContext {
	return c.connContext
}

func (c *UdpConn) Read() ([]byte, error) {
	if len(c.pkg) == 0 {
		return nil, errors.New("conn error: no data to read")
	}
	b := c.pkg[0]
	c.pkg = c.pkg[1:]
	return b, nil
}

func (c *UdpConn) Write(b []byte) error {
	_, err := c.raw.WriteToUDP(b, c.addr)
	return err
}

func (c *UdpConn) Close() {
	if c.closeCb != nil {
		c.closeCb(c.addr)
	}
}

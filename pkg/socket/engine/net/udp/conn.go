package udp

import (
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
)

type Conn struct {
	fd          int
	connContext *socket.ConnContext
	raw         *net.UDPConn
	pkg         [][]byte
	closeCb     func(identify string)
	localAddr   *net.UDPAddr
	remoteAddr  *net.UDPAddr
	closed      bool
	identify    string
}

func newConn(fd int, identify string, c *net.UDPConn, localAddr, remoteAddr *net.UDPAddr, ctx *socket.ConnContext, closeCb func(identify string)) *Conn {
	c.LocalAddr()
	return &Conn{
		fd:          fd,
		identify:    identify,
		connContext: ctx,
		raw:         c,
		localAddr:   localAddr,
		remoteAddr:  remoteAddr,
		closeCb:     closeCb,
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
	var addr string

	if v, ok := c.connContext.GetOptional("remote_addr"); ok {
		addr = v.(string)
	} else {
		addr = c.remoteAddr.String()
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return errors.New("invalid remote addr," + err.Error())
	}

	_, err = c.raw.WriteToUDP(b, udpAddr)
	return err
}

func (c *Conn) Close() {
	if c.closeCb != nil {
		c.closeCb(c.identify)
	}
	c.closed = true
}

func (c *Conn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

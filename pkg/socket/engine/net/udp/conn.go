package udp

import (
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
)

type Conn struct {
	fd               int
	connContext      *socket.ConnContext
	raw              *net.UDPConn
	pkg              [][]byte
	closeCb          func(identify string)
	localAddr        *net.UDPAddr
	remoteAddr       *net.UDPAddr
	closed           bool
	identify         string
	broadcastAddr    string
	writeInterceptor func(conn *Conn, data []byte) []byte
}

func newConn(fd int, identify, broadcastAddr string, c *net.UDPConn, localAddr, remoteAddr *net.UDPAddr, ctx *socket.ConnContext, closeCb func(identify string), writeInterceptor func(conn *Conn, data []byte) []byte) *Conn {
	return &Conn{
		fd:               fd,
		identify:         identify,
		broadcastAddr:    broadcastAddr,
		connContext:      ctx,
		raw:              c,
		localAddr:        localAddr,
		remoteAddr:       remoteAddr,
		closeCb:          closeCb,
		writeInterceptor: writeInterceptor,
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
	if c.writeInterceptor != nil {
		b = c.writeInterceptor(c, b)
		if len(b) == 0 {
			return nil
		}
	}
	var addr string
	if c.broadcastAddr != "" {
		addr = c.broadcastAddr
	} else {
		if v, ok := c.connContext.GetOptional("remote_addr"); ok {
			addr = v.(string)
		}
	}
	if addr != "" {
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return errors.New("invalid remote addr," + err.Error())
		}
		conn, err1 := net.DialUDP("udp", nil, udpAddr)
		if err1 != nil {
			return errors.New("dial udp failed," + err1.Error())
		}
		_, err = conn.Write(b)
		return err
	}

	_, err := c.raw.WriteToUDP(b, c.remoteAddr)
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

func (c *Conn) Identify() string {
	return c.identify
}

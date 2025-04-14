package mqtt

import (
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
)

type Conn struct {
	id          string
	fd          int
	connContext *socket.ConnContext
	local       Addr
	remote      Addr
	closeFn     func(id string)
	rq          [][]byte
	wfn         func(*Conn, []byte) error
}

func NewConn(id string, fd int, closeFn func(id string), wfn func(*Conn, []byte) error) *Conn {
	return &Conn{
		id:          id,
		fd:          fd,
		connContext: socket.NewContext(),
		local:       NewAddr("mqtt", "127.0.0.1"),
		remote:      NewAddr("mqtt", "0.0.0.0"),
		closeFn:     closeFn,
		wfn:         wfn,
	}
}

func (c *Conn) Id() string {
	return c.id
}

func (c *Conn) Fd() int {
	return c.fd
}

func (c *Conn) Context() *socket.ConnContext {
	return c.connContext
}

func (c *Conn) Read() ([]byte, error) {
	if len(c.rq) == 0 {
		return nil, nil
	}
	b := c.rq[0]
	c.rq = c.rq[1:]
	return b, nil
}

func (c *Conn) Write(b []byte) error {
	return c.wfn(c, b)
}

func (c *Conn) Close() {
	c.closeFn(c.id)
}

func (c *Conn) LocalAddr() net.Addr {
	return c.local
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *Conn) SetRq(b []byte) {
	c.rq = append(c.rq, b)
}

type Addr struct {
	network string
	address string
}

func NewAddr(network, address string) Addr {
	return Addr{network: network, address: address}
}

func (s Addr) Network() string {
	return s.network
}

func (s Addr) String() string {
	return s.address
}

package moc

import (
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
	"net/http"
)

type Conn struct {
	id          string
	connContext *socket.ConnContext
	local       Addr
	remote      Addr
	closed      bool
}

func NewConn(id string, local, remote Addr) *Conn {
	return &Conn{
		id:          id,
		connContext: socket.NewContext(),
		local:       local,
		remote:      remote,
	}
}

func NewHttpConn(id string, q *http.Request) *Conn {
	return NewConn(id, NewAddr("http", "127.0.0.1"), NewAddr("http", q.RemoteAddr))
}

func (c *Conn) Id() string {
	return c.id
}

func (c *Conn) Fd() int {
	return 0
}

func (c *Conn) Context() *socket.ConnContext {
	return c.connContext
}

func (c *Conn) Read() ([]byte, error) {
	return nil, nil
}

func (c *Conn) Write([]byte) error {
	return nil
}

func (c *Conn) Close() {
	c.closed = true
}

func (c *Conn) Closed() bool {
	return c.closed
}

func (c *Conn) LocalAddr() net.Addr {
	return c.local
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.remote
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

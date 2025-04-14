package http

import (
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
	"net/http"
)

type Conn struct {
	id          string
	fd          int
	connContext *socket.ConnContext
	local       Addr
	remote      Addr
	closeFn     func(id string)
	rq          []byte
	resp        []byte
	doing       bool
}

func NewConn(id string, fd int, q *http.Request, closeFn func(id string)) *Conn {
	return &Conn{
		id:          id,
		fd:          fd,
		connContext: socket.NewContext(),
		local:       NewAddr("http", "127.0.0.1"),
		remote:      NewAddr("http", q.RemoteAddr),
		closeFn:     closeFn,
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
	return c.rq, nil
}

func (c *Conn) Write(b []byte) error {
	if !c.doing {
		return errors.New("the connection not writeable")
	}
	c.resp = b
	return nil
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
	c.rq = b
	c.doing = true
}

func (c *Conn) Doing() bool {
	return c.doing
}

func (c *Conn) GetResp() []byte {
	b := c.resp
	c.resp = nil
	c.doing = false
	return b
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

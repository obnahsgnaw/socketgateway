package moc

import (
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
	"net/http"
)

type Conn struct {
	fd          int
	connContext *socket.ConnContext
	data        []byte
	local       Addr
	remote      Addr
}

func NewConn(fd int, local, remote Addr) *Conn {
	return &Conn{
		fd:          fd,
		connContext: socket.NewContext(),
		local:       local,
		remote:      remote,
	}
}

func NewHttpConn(fd int, q *http.Request) *Conn {
	return &Conn{
		fd:          fd,
		connContext: socket.NewContext(),
		local:       NewAddr("http", "127.0.0.1"),
		remote:      NewAddr("http", q.RemoteAddr),
	}
}

func (c *Conn) Fd() int {
	return 0
}

func (c *Conn) RawFd() int {
	return c.fd
}

func (c *Conn) Context() *socket.ConnContext {
	return c.connContext
}

func (c *Conn) Read() ([]byte, error) {
	return c.data, nil
}

func (c *Conn) Write(b []byte) error {
	c.data = b
	return nil
}

func (c *Conn) Close() {

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

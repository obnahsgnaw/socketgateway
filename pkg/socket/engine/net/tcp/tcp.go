package tcp

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
	"sync/atomic"
)

type Server struct {
	l            net.Listener
	network      string
	address      string
	fdProvider   func() int64
	index        int64
	onConnect    func(conn socket.Conn)
	onDisconnect func(conn socket.Conn, err error)
	onMessage    func(conn socket.Conn)
}

func New(address string, o ...Option) *Server {
	s := &Server{network: "tcp", address: address}
	s.fdProvider = func() int64 {
		atomic.AddInt64(&s.index, 1)
		return atomic.LoadInt64(&s.index)
	}
	s.onConnect = func(conn socket.Conn) {
		println(conn.Fd(), "connected", conn.RemoteAddr().String())
	}
	s.onDisconnect = func(conn socket.Conn, err error) {
		println(conn.Fd(), "disconnected", conn.RemoteAddr().String(), err.Error())
	}
	s.onMessage = func(conn socket.Conn) {
		b, _ := conn.Read()
		println(conn.Fd(), "message", string(b))
	}
	s.With(o...)
	return s
}

func (s *Server) With(o ...Option) {
	for _, oo := range o {
		if oo != nil {
			oo(s)
		}
	}
}

func (s *Server) Init() (err error) {
	s.l, err = net.Listen(s.network, s.address)
	return err
}

func (s *Server) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			break
		default:
			conn, err := s.l.Accept()
			if err == nil {
				fd := s.fdProvider()
				c := newConn(int(fd), conn, socket.NewContext())
				s.onConnect(c)
				go s.handConn(c)
			}
		}
	}
}

func (s *Server) handConn(c *Conn) {
	defer func() {
		c.Close()
		s.onDisconnect(c, errors.New("close by peer"))
	}()
	var buf [1024]byte
	for {
		n, err := c.raw.Read(buf[:])
		if err != nil {
			break
		}
		if n > 0 {
			c.pkg = append(c.pkg, buf[:n])
			c.Context().Active()
			s.onMessage(c)
		}
	}
}

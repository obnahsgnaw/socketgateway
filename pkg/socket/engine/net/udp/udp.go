package udp

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/net/udp/broadcastudp"
	"net"
	"sync/atomic"
)

type Server struct {
	l                 *net.UDPConn
	clients           map[string]*Conn
	network           string
	port              int
	localAddr         *net.UDPAddr
	fdProvider        func() int64
	index             int64
	onConnect         func(conn socket.Conn)
	onDisconnect      func(conn socket.Conn, err error)
	onMessage         func(conn socket.Conn)
	broadcastSendAddr string
	broadcastListen   bool
	identifyProvider  func([]byte) string
	bodyMax           int
}

func New(port int, o ...Option) *Server {
	s := &Server{
		l:       nil,
		clients: make(map[string]*Conn),
		network: "udp",
		port:    port,
		bodyMax: 1024,
	}
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
	s.onMessage = func(c socket.Conn) {
		println(c.Read())
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

func (s *Server) Init() error {
	s.localAddr = &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: s.port,
	}
	l, err := net.ListenUDP(s.network, s.localAddr)
	if err != nil {
		return err
	}
	if s.broadcastListen {
		// 获取文件描述符
		f, err1 := l.File()
		if err1 != nil {
			return err
		}
		fileDescriptor := f.Fd()
		if err = broadcastudp.Upgrade(fileDescriptor); err != nil {
			return err
		}
	}
	s.l = l
	return nil
}

func (s *Server) Run(ctx context.Context) {
	var data = make([]byte, s.bodyMax)
	for {
		select {
		case <-ctx.Done():
			break
		default:
			n, addr, err := s.l.ReadFromUDP(data)
			if err == nil && n > 0 {
				var fd int64
				var ok bool
				var identify = addr.String()
				var c *Conn
				pkg := data[:n]
				if s.identifyProvider != nil {
					identify = s.identifyProvider(pkg)
				}
				if identify != "" {
					if c, ok = s.clients[identify]; !ok {
						fd = s.fdProvider()
						c = newConn(int(fd), identify, s.broadcastSendAddr, s.l, s.localAddr, addr, socket.NewContext(), func(cc *Conn, ide string) {
							delete(s.clients, ide)
							s.onDisconnect(cc, errors.New("closed by server"))
						})
						s.clients[identify] = c
						s.clients[identify] = c
						s.onConnect(c)
						if c.closed { // 可能被连接中断
							break
						}
					}
					c.pkg = append(c.pkg, pkg)
					c.Context().Active()
					s.onMessage(c)
				}
			}
		}
	}
}

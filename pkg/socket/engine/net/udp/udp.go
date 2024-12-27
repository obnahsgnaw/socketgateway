package udp

import (
	"context"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"net"
	"sync/atomic"
	"syscall"
)

type Server struct {
	l                *net.UDPConn
	clients          map[string]int64
	network          string
	port             int
	localAddr        *net.UDPAddr
	fdProvider       func() int64
	index            int64
	onConnect        func(conn socket.Conn)
	onDisconnect     func(conn socket.Conn, err error)
	onMessage        func(conn socket.Conn)
	broadcast        bool
	broadcastAddr    string
	identifyProvider func([]byte) string
	readInterceptor  func(conn *Conn, data []byte) []byte
	writeInterceptor func(conn *Conn, data []byte) []byte
}

func New(port int, o ...Option) *Server {
	s := &Server{
		l:       nil,
		clients: make(map[string]int64),
		network: "udp",
		port:    port,
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
	if s.broadcast {
		if err = listenBroadcast(l); err != nil {
			return err
		}
	}
	s.l = l
	return nil
}

func (s *Server) Run(ctx context.Context) {
	var data = make([]byte, 1024*4)
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
				pkg := data[:n]
				if s.identifyProvider != nil {
					identify = s.identifyProvider(pkg)
					if identify == "" {
						identify = addr.String()
					}
				}
				if fd, ok = s.clients[identify]; !ok {
					fd = s.fdProvider()
					s.clients[identify] = fd
				}
				c := newConn(int(fd), identify, s.broadcastAddr, s.l, s.localAddr, addr, socket.NewContext(), func(ide string) {
					delete(s.clients, ide)
				}, s.writeInterceptor)
				if !ok {
					s.onConnect(c)
					if c.closed { // 可能被连接中断
						break
					}
				}
				if s.readInterceptor != nil {
					pkg = s.readInterceptor(c, pkg)
				}
				if len(pkg) > 0 {
					c.pkg = append(c.pkg, pkg)
					c.Context().Active()
					s.onMessage(c)
				}
			}
		}
	}
}

func listenBroadcast(conn *net.UDPConn) error {
	// 获取文件描述符
	f, err := conn.File()
	if err != nil {
		return err
	}
	fileDescriptor := f.Fd()

	// 设置 socket 选项为广播模式 (SO_BROADCAST)
	err = syscall.SetsockoptInt(int(fileDescriptor), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	return err
}

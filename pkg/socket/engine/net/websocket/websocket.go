package websocket

import (
	"context"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

type Server struct {
	port         int
	upgrader     websocket.Upgrader
	fdProvider   func() int64
	index        int64
	onConnect    func(conn socket.Conn)
	onDisconnect func(conn socket.Conn, err error)
	onMessage    func(conn socket.Conn)
}

func New(port int, o ...Option) *Server {
	s := &Server{
		port: port,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  2048,
			WriteBufferSize: 2048,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
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

func (s *Server) Init() error {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := s.upgrader.Upgrade(writer, request, nil)
		if err != nil {
			_, _ = writer.Write([]byte(err.Error()))
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		fd := s.fdProvider()
		c := newWssConn(int(fd), conn, socket.NewContext())
		s.onConnect(c)
		if !c.closed {
			s.handConn(c)
		}
	})

	return nil
}

func (s *Server) Run(ctx context.Context) {
	server := &http.Server{Addr: ":" + strconv.Itoa(s.port), Handler: nil}
	go func() {
		for {
			select {
			case <-ctx.Done():
				ctx1, cel := context.WithTimeout(ctx, time.Second*5)
				if err := server.Shutdown(ctx1); err != nil {
					log.Println(err.Error())
				}
				cel()
				break
			}
		}
	}()
	log.Fatal(server.ListenAndServe())
}

func (s *Server) handConn(c *Conn) {
	defer func() {
		c.Close()
		s.onDisconnect(c, errors.New("close by peer"))
	}()
	for {
		_, b, err := c.raw.ReadMessage()
		if err != nil {
			break
		}
		if len(b) > 0 {
			c.pkg = append(c.pkg, b)
			c.Context().Active()
			s.onMessage(c)
		}
	}
}

package socket

import (
	"context"
	"github.com/obnahsgnaw/socketgateway/pkg/group"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"sync"
)

// Server ---> event ---> tcp engine

type Engine interface {
	Run(ctx context.Context, s *Server, eventHandler Event, t sockettype.SocketType, port int, config *Config) (err error)
	Stop() error
}

type Config struct {
	MultiCore bool
	Keepalive uint
	NoDelay   bool
	Ticker    bool
	ReuseAddr bool
}

// Server 服务
type Server struct {
	ctx         context.Context
	typ         sockettype.SocketType
	port        int
	event       Event
	config      *Config
	groups      *group.Groups
	engine      Engine
	connections sync.Map // map[int]Conn
	connIdBinds sync.Map // map[string]int
}

// New return a Server
func New(ctx context.Context, t sockettype.SocketType, p int, e Engine, event Event, c *Config) *Server {
	s := &Server{
		ctx:    ctx,
		typ:    t,
		port:   p,
		engine: e,
		config: c,
		groups: group.New(),
	}
	s.event = newCountedEvent(s, event)

	return s
}

func (s *Server) Context() context.Context {
	return s.ctx
}

func (s *Server) Type() sockettype.SocketType {
	return s.typ
}

func (s *Server) Port() int {
	return s.port
}

func (s *Server) Event() Event {
	return s.event
}

func (s *Server) Config() *Config {
	return s.config
}

func (s *Server) Engine() Engine {
	return s.engine
}

func (s *Server) Groups() *group.Groups {
	return s.groups
}

func (s *Server) Start() error {
	return s.engine.Run(s.ctx, s, s.event, s.typ, s.port, s.config)
}

func (s *Server) SyncStart(cb func(err error)) {
	go func(s *Server) {
		defer s.Stop()
		if err := s.Start(); err != nil {
			cb(err)
		}
	}(s)
}

func (s *Server) Stop() {
	_ = s.engine.Stop()
	s.RangeConnections(func(c Conn) bool {
		c.Close()
		return true
	})
}

func (s *Server) BindId(c Conn, id ConnId) {
	if id.Type != "" && id.Id != "" {
		c.Context().bind(id)
		s.connIdBinds.Store(id.String(), c.Fd())
	}
}

func (s *Server) UnbindId(c Conn, id ConnId) {
	if id.Type != "" && id.Id != "" {
		s.connIdBinds.Delete(id.String())
		c.Context().unbind(id)
	}
}

func (s *Server) GetFdConn(fd int) Conn {
	if c, ok := s.connections.Load(fd); ok {
		return c.(Conn)
	}
	return nil
}

func (s *Server) GetIdConn(id ConnId) Conn {
	if fd, ok := s.connIdBinds.Load(id.String()); ok {
		if c := s.GetFdConn(fd.(int)); c != nil {
			return c
		} else {
			s.connIdBinds.Delete(id.String())
			return nil
		}
	}
	return nil
}

func (s *Server) RangeConnections(f func(c Conn) bool) {
	s.connections.Range(func(key, value interface{}) bool {
		return f(value.(Conn))
	})
}

func (s *Server) addConn(c Conn) {
	s.connections.Store(c.Fd(), c)
}

func (s *Server) delConn(c Conn) {
	s.connections.Delete(c.Fd())
	c.Context().RangeId(func(id ConnId) {
		s.connIdBinds.Delete(id.String())
	})
}

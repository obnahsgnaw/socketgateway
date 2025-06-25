package socket

import (
	"context"
	"github.com/obnahsgnaw/rpc/pkg/rpcclient"
	"github.com/obnahsgnaw/socketgateway/pkg/group"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"strconv"
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
	ctx          context.Context
	typ          sockettype.SocketType
	port         int
	event        Event
	config       *Config
	groups       *group.Groups
	engine       Engine
	connections  sync.Map // map[int]Conn
	connIdBinds  *IDManager
	relatedBinds *IDManager
	watchClient  *rpcclient.Manager
}

// New return a Server
func New(ctx context.Context, t sockettype.SocketType, p int, e Engine, event Event, c *Config, watchClient *rpcclient.Manager) *Server {
	s := &Server{
		ctx:          ctx,
		typ:          t,
		port:         p,
		engine:       e,
		config:       c,
		groups:       group.New(),
		watchClient:  watchClient,
		connIdBinds:  newIDManager(),
		relatedBinds: newIDManager(),
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
	if id.Type != "" && id.Id != "" && c.Fd() > 0 {
		c.Context().bind(id)
		s.connIdBinds.Add(id.String(), c.Fd())
	}
}

func (s *Server) UnbindId(c Conn, id ConnId) {
	if id.Type != "" && id.Id != "" {
		s.delIdFd(id, c.Fd())
		c.Context().unbind(id)
	}
}

func (s *Server) UnbindTypedId(c Conn, typ string) {
	if typ != "" {
		id := c.Context().TypedId(typ)
		if id.Id != "" {
			s.delIdFd(id, c.Fd())
			c.Context().unbind(id)
		}
	}
}

func (s *Server) delIdFd(id ConnId, fd int) {
	s.connIdBinds.Del(id.String(), fd)
}

func (s *Server) GetFdConn(fd int) Conn {
	if c, ok := s.connections.Load(fd); ok {
		return c.(Conn)
	}
	return nil
}

func (s *Server) GetIdConn(id ConnId) (list []Conn) {
	fds := s.connIdBinds.Get(id.String())
	for _, fd := range fds {
		if c := s.GetFdConn(fd); c != nil {
			list = append(list, c)
		} else {
			s.connIdBinds.Del(id.String(), fd)
		}
	}
	return
}

func (s *Server) RangeConnections(f func(c Conn) bool) {
	s.connections.Range(func(key, value interface{}) bool {
		return f(value.(Conn))
	})
}

func (s *Server) addConn(c Conn) {
	if c.Fd() > 0 {
		s.connections.Store(c.Fd(), c)
	}
}

func (s *Server) delConn(c Conn) {
	if c.Fd() > 0 {
		s.connections.Delete(c.Fd())
		c.Context().RangeId(func(id ConnId) {
			s.delIdFd(id, c.Fd())
		})
		// 退组
		s.Groups().RangeGroups(func(g *group.Group) bool {
			g.Leave(c.Fd())
			return true
		})
	}
}

func (s *Server) Auth(c Conn, u *AuthUser) {
	if u != nil {
		c.Context().auth(u)
		s.BindId(c, ConnId{
			Id:   strconv.Itoa(int(u.Id)),
			Type: "UID",
		})
	} else {
		u1 := c.Context().User()
		c.Context().auth(u)
		if u1 != nil {
			s.UnbindId(c, ConnId{
				Id:   strconv.Itoa(int(u1.Id)),
				Type: "UID",
			})
		}
	}
}

func (s *Server) Authenticate(c Conn, u *Authentication) error {
	if u != nil {
		c.Context().authenticate(u)
		s.BindId(c, ConnId{
			Id:   u.Id,
			Type: "TARGET",
		})
		s.BindId(c, ConnId{
			Id:   u.Sn,
			Type: "SN",
		})
	} else {
		u1 := c.Context().Authentication()
		c.Context().authenticate(u)
		if u1 != nil {
			s.UnbindId(c, ConnId{
				Id:   u1.Id,
				Type: "TARGET",
			})
			s.UnbindId(c, ConnId{
				Id:   u1.Sn,
				Type: "SN",
			})
		}
	}
	return nil
}

func (s *Server) ClearAuthentication(c Conn) {
	_ = s.Authenticate(c, nil)
}

func (s *Server) GetAuthenticatedConn(id string) (list []Conn) {
	return s.GetIdConn(ConnId{Id: id, Type: "TARGET"})
}

func (s *Server) GetAuthenticatedSnConn(id string) (list []Conn) {
	return s.GetIdConn(ConnId{Id: id, Type: "SN"})
}

func (s *Server) GwManager() *rpcclient.Manager {
	return s.watchClient
}

func (s *Server) BindProxyTarget(target string, fd int) {
	s.relatedBinds.Add(target, fd)
}

func (s *Server) UnbindProxyTarget(target string, fd int) {
	s.relatedBinds.Del(target, fd)
}

func (s *Server) QueryProxyTargetBinds(target string) []int {
	return s.relatedBinds.Get(target)
}

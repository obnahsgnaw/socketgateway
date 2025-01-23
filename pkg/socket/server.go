package socket

import (
	"context"
	"github.com/obnahsgnaw/rpc/pkg/rpcclient"
	"github.com/obnahsgnaw/socketgateway/pkg/group"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"strconv"
	"strings"
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
	connIdBinds  sync.Map // map[string]map[int]struct{}
	relatedBinds sync.Map // map[string][]string
	watchClient  *rpcclient.Manager
}

// New return a Server
func New(ctx context.Context, t sockettype.SocketType, p int, e Engine, event Event, c *Config, watchClient *rpcclient.Manager) *Server {
	s := &Server{
		ctx:         ctx,
		typ:         t,
		port:        p,
		engine:      e,
		config:      c,
		groups:      group.New(),
		watchClient: watchClient,
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
		if v, ok := s.connIdBinds.Load(id.String()); ok {
			vv := v.(map[int]struct{})
			vv[c.Fd()] = struct{}{}
			s.connIdBinds.Store(id.String(), vv)
		} else {
			s.connIdBinds.Store(id.String(), map[int]struct{}{c.Fd(): {}})
		}
	}
}

func (s *Server) UnbindId(c Conn, id ConnId) {
	if id.Type != "" && id.Id != "" {
		if v, ok := s.connIdBinds.Load(id.String()); ok {
			vv := v.(map[int]struct{})
			delete(vv, c.Fd())
			if len(vv) == 0 {
				s.connIdBinds.Delete(id.String())
			} else {
				s.connIdBinds.Store(id.String(), vv)
			}
		}
		c.Context().unbind(id)
	}
}

func (s *Server) UnbindTypedId(c Conn, typ string) {
	if typ != "" {
		id := c.Context().TypedId(typ)
		if id.Id != "" {
			if v, ok := s.connIdBinds.Load(id.String()); ok {
				vv := v.(map[int]struct{})
				delete(vv, c.Fd())
				if len(vv) == 0 {
					s.connIdBinds.Delete(id.String())
				} else {
					s.connIdBinds.Store(id.String(), vv)
				}
			}
			c.Context().unbind(id)
		}
	}
}

func (s *Server) GetFdConn(fd int) Conn {
	if c, ok := s.connections.Load(fd); ok {
		return c.(Conn)
	}
	return nil
}

func (s *Server) GetIdConn(id ConnId) (list []Conn) {
	if v, ok := s.connIdBinds.Load(id.String()); ok {
		fds := v.(map[int]struct{})
		for fd := range fds {
			if c := s.GetFdConn(fd); c != nil {
				list = append(list, c)
			} else {
				delete(fds, fd)
			}
		}
		if len(fds) == 0 {
			s.connIdBinds.Delete(id.String())
		} else {
			s.connIdBinds.Store(id.String(), fds)
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
			if v, ok := s.connIdBinds.Load(id.String()); ok {
				vv := v.(map[int]struct{})
				delete(vv, c.Fd())
				if len(vv) == 0 {
					s.connIdBinds.Delete(id.String())
				} else {
					s.connIdBinds.Store(id.String(), vv)
				}
			}
		})
		au := c.Context().Authentication()
		if au != nil {
			s.UnbindRelate(c, au.Master, au.Id)
		}
		// 退组
		s.Groups().RangeGroups(func(g *group.Group) bool {
			g.Leave(c.Fd())
			return true
		})
	}
}

func (s *Server) BindRelate(c Conn, master, id string) {
	if v, ok := s.relatedBinds.Load(master); ok {
		vv := v.([]string)
		vv = append(vv, id+"@"+strconv.Itoa(c.Fd()))
		s.relatedBinds.Store(master, vv)
	} else {
		s.relatedBinds.Store(master, []string{id + "@" + strconv.Itoa(c.Fd())})
	}
}

func (s *Server) UnbindRelate(c Conn, master, id string) {
	if v, ok := s.relatedBinds.Load(master); ok {
		vv := v.([]string)
		var vv1 []string
		for _, v1 := range vv {
			if v1 != id+"@"+strconv.Itoa(c.Fd()) {
				vv1 = append(vv1, v1)
			}
		}
		s.relatedBinds.Store(master, vv1)
	}
}

func (s *Server) GetRelatedConn(master string) []Conn {
	var cc []Conn
	if v, ok := s.relatedBinds.Load(master); ok {
		vv := v.([]string)
		for _, v1 := range vv {
			v2 := strings.Split(v1, "@")[0]
			c1 := s.GetIdConn(ConnId{Id: v2, Type: "TARGET"})
			if c1 != nil {
				cc = append(cc, c1...)
			}
		}
	}
	return cc
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

func (s *Server) Authenticate(c Conn, u *Authentication) {
	if u != nil {
		c.Context().authenticate(u)
		s.BindId(c, ConnId{
			Id:   u.Id,
			Type: "TARGET",
		})
		s.BindRelate(c, u.Master, u.Id)
	} else {
		u1 := c.Context().Authentication()
		c.Context().authenticate(u)
		if u1 != nil {
			s.UnbindId(c, ConnId{
				Id:   u1.Id,
				Type: "TARGET",
			})
			s.UnbindRelate(c, u1.Master, u1.Id)
		}
	}
}

func (s *Server) GetAuthenticatedConn(id string) (list []Conn) {
	cc := ConnId{Id: id, Type: "TARGET"}
	if v, ok := s.connIdBinds.Load(cc.String()); ok {
		fds := v.(map[int]struct{})
		for fd := range fds {
			if c := s.GetFdConn(fd); c != nil {
				list = append(list, c)
			} else {
				delete(fds, fd)
			}
		}
		if len(fds) == 0 {
			s.connIdBinds.Delete(cc.String())
		} else {
			s.connIdBinds.Store(cc.String(), fds)
		}
	}
	return nil
}

func (s *Server) GwManager() *rpcclient.Manager {
	return s.watchClient
}

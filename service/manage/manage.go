package manage

import (
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/group"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketutil/codec"
	"strconv"
	"time"
)

// Manager the connection
type Manager struct {
	s                 *socket.Server
	config            *GwConfig
	protocol          string
	onofflineListener func(*Connection, bool)
	fdMessageListener func(*Message)
	sender            func(c socket.Conn, act codec.Action, pkg []byte) error
}

type Gateway struct {
	Protocol string    `json:"protocol"`
	Config   *GwConfig `json:"config"`
}

type GwConfig struct {
	Port              int    `json:"port"`
	RpcPort           int    `json:"rpc_port"`
	DocDisable        bool   `json:"doc_disable"`
	KeepaliveInterval uint   `json:"keepalive_interval"`
	AuthCheckInterval uint   `json:"auth_check_interval"`
	HeartbeatInterval uint   `json:"heartbeat_interval"`
	TickInterval      uint   `json:"tick_interval"`
	Security          bool   `json:"security"`
	SecurityType      string `json:"security_type"`
	SecurityEncoder   string `json:"security_encoder"`
	SecurityEncode    bool   `json:"security_encode"`
}

type Connection struct {
	Fd             int       `json:"fd"`
	RemoteAddr     string    `json:"remote_addr"`
	RemotePort     int       `json:"remote_port"`
	ConnectedAt    time.Time `json:"connected_at"`
	ActiveAt       time.Time `json:"active_at"`
	Authentication *socket.Authentication
	User           *socket.AuthUser
	Binds          map[string]string
}

func New(protocol string, s *socket.Server, config *GwConfig, sender func(c socket.Conn, act codec.Action, pkg []byte) error) *Manager {
	m := &Manager{
		protocol: protocol,
		s:        s,
		config:   config,
		sender:   sender,
	}
	return m
}

func (m *Manager) Gateway() *Gateway {
	return &Gateway{
		Protocol: m.protocol,
		Config:   m.config,
	}
}

func (m *Manager) Connections() (list []*Connection) {
	m.s.RangeConnections(func(c socket.Conn) bool {
		list = append(list, m.toConnection(c))
		return true
	})
	return
}

func (m *Manager) ConnectionsListen(fn func(*Connection, bool)) {
	m.onofflineListener = fn
}

func (m *Manager) MessagesListen(fn func(*Message)) {
	m.fdMessageListener = fn
}

func (m *Manager) MessageSend(fd int, act codec.Action, pkg []byte) (err error) {
	conn := m.s.GetFdConn(fd)
	if conn == nil {
		err = errors.New("connection not found")
		return
	}
	err = m.sender(conn, act, pkg)
	return
}

func (m *Manager) Groups() (list []string) {
	m.s.Groups().RangeGroups(func(g *group.Group) bool {
		list = append(list, g.Name())
		return true
	})
	return
}

func (m *Manager) GroupMembers(name string) (list []string) {
	m.s.Groups().GetGroup(name).RangeMembers(func(fd int, id string) bool {
		list = append(list, id+"["+strconv.Itoa(fd)+"]")
		return true
	})
	return
}

func (m *Manager) toConnection(c socket.Conn) *Connection {
	return &Connection{
		Fd:             c.Fd(),
		RemoteAddr:     c.RemoteAddr().String(),
		RemotePort:     0,
		ConnectedAt:    c.Context().ConnectedAt(),
		ActiveAt:       c.Context().LastActiveAt(),
		Authentication: c.Context().Authentication(),
		User:           c.Context().User(),
		Binds:          c.Context().IdMap(),
	}
}

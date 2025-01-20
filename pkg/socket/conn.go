package socket

import (
	"net"
	"sync"
	"time"
)

type Conn interface {
	Fd() int
	Context() *ConnContext
	Read() ([]byte, error)
	Write([]byte) error
	Close()
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

// ConnContext 连接conn的数据绑定
type ConnContext struct {
	connectedAt    time.Time
	lastActiveAt   time.Time
	ids            map[string]ConnId // type=>ConnId
	upgraded       bool
	authUser       *AuthUser
	optional       sync.Map
	authentication *Authentication
}

// ConnId id绑定信息
type ConnId struct {
	Id   string
	Type string
}

type AuthUser struct {
	Id   uint
	Name string
	Attr map[string]string
}

type Authentication struct {
	Type   string
	Id     string
	Master string
	Cid    uint32
	Uid    uint32
}

func (s *AuthUser) GetAttr(key, defVal string) string {
	if v, ok := s.Attr[key]; ok {
		return v
	}
	return defVal
}

func (b ConnId) String() string {
	return b.Type + ":" + b.Id
}

func NewContext() *ConnContext {
	return &ConnContext{
		connectedAt:  time.Now(),
		lastActiveAt: time.Now(),
		ids:          make(map[string]ConnId),
		upgraded:     false,
		authUser:     nil,
		optional:     sync.Map{},
	}
}

func (c *ConnContext) ConnectedAt() time.Time {
	return c.connectedAt
}

func (c *ConnContext) Active() {
	c.lastActiveAt = time.Now()
}

func (c *ConnContext) LastActiveAt() time.Time {
	return c.lastActiveAt
}

func (c *ConnContext) bind(id ConnId) {
	if id.Type != "" && id.Id != "" {
		c.ids[id.Type] = id
	}
}

func (c *ConnContext) unbind(id ConnId) {
	if id.Type != "" && id.Id != "" {
		if _, ok := c.ids[id.Type]; ok {
			delete(c.ids, id.Type)
		}
	}
}

func (c *ConnContext) bond(id ConnId) bool {
	if id.Type != "" && id.Id != "" {
		if _, ok := c.ids[id.Type]; ok {
			return true
		}
	}

	return false
}

func (c *ConnContext) Ids() map[string]ConnId {
	return c.ids
}

func (c *ConnContext) IdMap() map[string]string {
	m := make(map[string]string)
	for _, id := range c.ids {
		if id.Type != "TARGET" && id.Type != "UID" { // 减少传输
			m[id.Type] = id.Id
		}
	}
	return m
}

func (c *ConnContext) Id() ConnId {
	if len(c.ids) >= 0 {
		for _, id := range c.ids {
			return id
		}
	}
	return ConnId{}
}

func (c *ConnContext) RangeId(h func(id ConnId)) {
	if len(c.ids) >= 0 && h != nil {
		for _, id := range c.ids {
			h(id)
		}
	}
}

func (c *ConnContext) TypedId(typ string) ConnId {
	id, ok := c.ids[typ]
	if ok {
		return id
	}
	return ConnId{}
}

func (c *ConnContext) Upgrade() {
	c.upgraded = true
}

func (c *ConnContext) Upgraded() bool {
	return c.upgraded
}

func (c *ConnContext) auth(u *AuthUser) {
	c.authUser = u
}

func (c *ConnContext) Authed() bool {
	return c.authUser != nil
}

func (c *ConnContext) User() *AuthUser {
	return c.authUser
}

func (c *ConnContext) Authentication() *Authentication {
	return c.authentication
}

func (c *ConnContext) Authenticated() bool {
	return c.authentication != nil
}

func (c *ConnContext) authenticate(a *Authentication) {
	c.authentication = a
}

// GetOptional 返回自定义的数据
func (c *ConnContext) GetOptional(key interface{}) (v interface{}, ok bool) {
	return c.optional.Load(key)
}

func (c *ConnContext) GetAndDelOptional(key interface{}) (v interface{}, ok bool) {
	return c.optional.LoadAndDelete(key)
}

// SetOptional 设置自定义的数据
func (c *ConnContext) SetOptional(key, v interface{}) {
	c.optional.Store(key, v)
}

// DelOptional 删除自定义的数据
func (c *ConnContext) DelOptional(key interface{}) {
	c.optional.Delete(key)
}

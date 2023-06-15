package socket

import (
	"sync"
	"time"
)

type Conn interface {
	Fd() int
	Context() *ConnContext
	Read() ([]byte, error)
	Write([]byte) error
	Close()
}

// ConnContext 连接conn的数据绑定
type ConnContext struct {
	connectedAt  time.Time
	lastActiveAt time.Time
	id           ConnId
	upgraded     bool
	authed       bool
	optional     sync.Map
}

// ConnId id绑定信息
type ConnId struct {
	Id   string
	Type string
}

func NewContext() *ConnContext {
	return &ConnContext{
		connectedAt:  time.Now(),
		lastActiveAt: time.Now(),
		id:           ConnId{},
		upgraded:     false,
		authed:       false,
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
	c.id = id
}

func (c *ConnContext) Id() ConnId {
	return c.id
}

func (c *ConnContext) Upgrade() {
	c.upgraded = true
}

func (c *ConnContext) Upgraded() bool {
	return c.upgraded
}

func (c *ConnContext) auth() {
	c.authed = true
}

func (c *ConnContext) Authed() bool {
	return c.authed
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

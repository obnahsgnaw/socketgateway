package group

import (
	"github.com/panjf2000/gnet/v2/pkg/pool/goroutine"
	"sync"
)

type (
	Group struct {
		name    string
		members sync.Map // map[int]string
		gs      *Groups
	}
	Groups struct {
		members sync.Map // map[string]*Group
	}
)

func New() *Groups {
	return &Groups{}
}

func (gs *Groups) GetGroup(name string) *Group {
	if g, ok := gs.members.Load(name); ok {
		return g.(*Group)
	}
	g := newGroup(name)
	g.gs = gs
	gs.members.Store(name, g)

	return g
}

func (gs *Groups) DelGroup(g *Group) {
	gs.members.Delete(g.name)
}

func (gs *Groups) RangeGroups(f func(g *Group) bool) {
	gs.members.Range(func(key, value interface{}) bool {
		return f(value.(*Group))
	})
}

func newGroup(name string) *Group {
	return &Group{
		name: name,
	}
}

func (g *Group) Name() string {
	return g.name
}

func (g *Group) Join(fd int, id string) {
	g.members.Store(fd, id)
}

func (g *Group) Leave(fd int) {
	g.members.Delete(fd)
}

func (g *Group) Broadcast(handle func(fd int, id string)) {
	pool := goroutine.Default()
	defer pool.Release()

	var wg sync.WaitGroup
	g.RangeMembers(func(fd int, id string) bool {
		wg.Add(1)
		_ = pool.Submit(func(fd1 int, id1 string) func() {
			return func() {
				handle(fd1, id1)
				wg.Done()
			}
		}(fd, id))
		return true
	})
	wg.Wait()
}

func (g *Group) Destroy() {
	g.gs.DelGroup(g)
}

func (g *Group) RangeMembers(f func(fd int, id string) bool) {
	g.members.Range(func(key, value interface{}) bool {
		return f(key.(int), value.(string))
	})
}

package action

import (
	"github.com/obnahsgnaw/socketutil/codec"
	"testing"
)

func TestManager(t *testing.T) {
	m := NewManager()

	m.RegisterRemoteAction(codec.NewAction(1, "a"), "10.10.11.123:8081", "7001")
	m.RegisterRemoteAction(codec.NewAction(1, "a"), "10.10.11.123:8082", "7002")
	m.RegisterRemoteAction(codec.NewAction(1, "a"), "10.10.11.123:8083", "7003")

	s := m.getFlbServer(8, codec.ActionId(1))

	println(s)
}

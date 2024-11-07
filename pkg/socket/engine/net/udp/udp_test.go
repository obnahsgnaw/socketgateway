package udp

import (
	"context"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"testing"
)

func TestUdp(t *testing.T) {
	u := New(1821, Message(func(c socket.Conn) {
		b, _ := c.Read()
		println(string(b))
	}))
	if err := u.Init(); err != nil {
		t.Error(err)
	}
	u.Run(context.Background())
}

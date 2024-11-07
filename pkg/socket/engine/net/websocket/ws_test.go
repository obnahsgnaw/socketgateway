package websocket

import (
	"context"
	"testing"
)

func TestWs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ws := New(1803)
	if err := ws.Init(); err != nil {
		t.Error(err)
	}
	ws.Run(ctx)
}

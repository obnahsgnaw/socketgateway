package tcp

import (
	"context"
	"testing"
)

func TestTcp(t *testing.T) {
	tp := New("127.0.0.1:8001")

	if err := tp.Init(); err != nil {
		t.Error(err)
	}
	tp.Run(context.Background())
}

package limiter

import (
	"testing"
	"time"
)

func TestTimeLimiter(t *testing.T) {
	tt := NewTimeLimiter(2)
	target := "test"

	if !tt.Access(target) {
		t.Error("need access but not")
		return
	}
	tt.Hit(target)

	if tt.Access(target) {
		t.Error("need not access but can")
		return
	}
	time.Sleep(time.Second * 2)

	if !tt.Access(target) {
		t.Error("need access but not")
		return
	}
	tt.Hit(target)
	tt.Hit(target)
	time.Sleep(time.Second * 2)

	if tt.Access(target) {
		t.Error("need not access but can")
		return
	}
	time.Sleep(time.Second * 2)

	if !tt.Access(target) {
		t.Error("need access but not")
		return
	}

}

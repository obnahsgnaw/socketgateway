package socket

import (
	"fmt"
	"testing"
	"time"
)

func TestBind(t *testing.T) {
	b := newIDManager()

	for i := 0; i < 10; i++ {
		go func(index int) {
			for {
				b.Add("test", index)
				time.Sleep(time.Millisecond * 100 * time.Duration(index))
				b.Del("test", index)
			}
		}(i)
	}

	for {
		fmt.Printf("%v\n", b.Get("test"))
		time.Sleep(time.Second)
	}
	select {}
}

package limiter

import (
	"sync"
	"time"
)

type TimeLimiter struct {
	targets  sync.Map // target => expire timestamp
	interval int64
}

type limit struct {
	expireAt      int64
	magnification int64
}

func NewTimeLimiter(interval int64) *TimeLimiter {
	return &TimeLimiter{
		interval: interval,
	}
}

func (al *TimeLimiter) Hit(target string) {
	if v, ok := al.targets.Load(target); ok {
		vv := v.(limit)
		now := time.Now().Unix()
		if vv.expireAt < now {
			vv.expireAt = now
		}
		vv.magnification *= 2
		vv.expireAt = vv.expireAt + al.interval*vv.magnification
		al.targets.Store(target, vv)
		return
	}
	al.targets.Store(target, limit{
		expireAt:      time.Now().Unix() + al.interval,
		magnification: 1,
	})
}

func (al *TimeLimiter) Release(target string) {
	al.targets.Delete(target)
}

func (al *TimeLimiter) Access(target string) bool {
	if v, ok := al.targets.Load(target); ok {
		vv := v.(limit)
		now := time.Now().Unix()
		return now >= vv.expireAt
	}
	return true
}

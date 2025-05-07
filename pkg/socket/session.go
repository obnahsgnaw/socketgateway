package socket

import (
	"strconv"
	"sync"
	"time"
)

type SessionManager struct {
	sessions sync.Map // target => session
}

type Session struct {
	Id       string
	ttl      int
	num      int
	ExpireAt *time.Time
}

func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

func (s *SessionManager) Get(target string) (string, bool) {
	if session, ok := s.sessions.Load(target); ok {
		ss := session.(*Session)
		if ss.ExpireAt != nil && time.Now().After(*ss.ExpireAt) {
			s.sessions.Delete(target)
		} else {
			return ss.Id, true
		}
	}
	return "", false
}

func (s *SessionManager) GetActive(target string) (string, bool) {
	if session, ok := s.sessions.Load(target); ok {
		ss := session.(*Session)
		if ss.ExpireAt != nil {
			if time.Now().After(*ss.ExpireAt) {
				s.sessions.Delete(target)
			}
			return "", false
		} else {
			return ss.Id, true
		}
	}
	return "", false
}

func (s *SessionManager) New(target string, ttl int) string {
	sid := s.genSessionId()
	s.sessions.Store(target, &Session{
		Id:  sid,
		ttl: ttl,
		num: 1,
	})
	return sid
}

func (s *SessionManager) AddNum(target string) {
	if session, ok := s.sessions.Load(target); ok {
		ss := session.(*Session)
		ss.num++
	}
}

func (s *SessionManager) Delete(target string) {
	if session, ok := s.sessions.Load(target); ok {
		ss := session.(*Session)
		if ss.num > 1 {
			ss.num--
		} else {
			if ss.ttl == 0 {
				s.sessions.Delete(target)
			} else {
				expireAt := time.Now().Add(time.Duration(ss.ttl) * time.Second)
				ss.ExpireAt = &expireAt
				s.sessions.Store(target, ss)
			}
		}
	}
}

func (s *SessionManager) genSessionId() string {
	return "sid" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

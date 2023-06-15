package socket

import (
	"time"
)

// Event 事件处理器接口
type Event interface {
	OnBoot(s *Server)

	OnShutdown(s *Server)

	OnOpen(s *Server, c Conn)

	OnClose(s *Server, c Conn, err error)

	OnTraffic(s *Server, c Conn)

	OnTick(s *Server) (delay time.Duration)
}

// BuiltinEvent 内建事件处理器
type BuiltinEvent struct{}

func (es *BuiltinEvent) OnBoot(_ *Server) {}

func (es *BuiltinEvent) OnShutdown(_ *Server) {}

func (es *BuiltinEvent) OnOpen(_ *Server, _ Conn) {}

func (es *BuiltinEvent) OnClose(_ *Server, _ Conn, _ error) {}

func (es *BuiltinEvent) OnTraffic(_ *Server, _ Conn) {}

func (es *BuiltinEvent) OnTick(_ *Server) (delay time.Duration) {
	return
}

type countedEvent struct {
	s *Server
	e Event
}

func newCountedEvent(s *Server, e Event) *countedEvent {
	return &countedEvent{
		s: s,
		e: e,
	}
}

func (s *countedEvent) OnBoot(ss *Server) {
	s.e.OnBoot(ss)
}

func (s *countedEvent) OnShutdown(ss *Server) {
	s.e.OnShutdown(ss)
}

func (s *countedEvent) OnOpen(ss *Server, c Conn) {
	s.s.addConn(c)
	s.e.OnOpen(ss, c)
}

func (s *countedEvent) OnClose(ss *Server, c Conn, e error) {
	s.s.delConn(c)
	s.e.OnClose(ss, c, e)
}

func (s *countedEvent) OnTraffic(ss *Server, c Conn) {
	s.e.OnTraffic(ss, c)
}

func (s *countedEvent) OnTick(ss *Server) (delay time.Duration) {
	return s.e.OnTick(ss)
}

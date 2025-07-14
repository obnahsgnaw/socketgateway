package manage

import "github.com/obnahsgnaw/socketgateway/pkg/socket"

// Trigger for event-handler to trigger event
type Trigger struct {
	s *Manager
}

func NewTrigger(m *Manager) *Trigger {
	return &Trigger{m}
}

func (m *Trigger) ConnectionJoin(c socket.Conn) {
	if m.s.onofflineListener != nil {
		m.s.onofflineListener(m.s.toConnection(c), true)
	}
}

func (m *Trigger) ConnectionLeave(c socket.Conn) {
	if m.s.onofflineListener != nil {
		m.s.onofflineListener(m.s.toConnection(c), false)
	}
}

func (m *Trigger) ConnectionReceivedMessage(c socket.Conn, level string, desc string, data []byte) {
	if m.s.fdMessageListener != nil {
		m.s.fdMessageListener(&Message{
			Fd:      c.Fd(),
			Type:    Receive,
			Level:   level,
			Desc:    desc,
			Package: data,
		})
	}
}

func (m *Trigger) ConnectionSentMessage(c socket.Conn, level string, desc string, data []byte) {
	if m.s.fdMessageListener != nil {
		m.s.fdMessageListener(&Message{
			Fd:      c.Fd(),
			Type:    Send,
			Level:   level,
			Desc:    desc,
			Package: data,
		})
	}
}

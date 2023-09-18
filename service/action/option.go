package action

import (
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/socketutil/codec"
)

type Option func(s *Manager)

func CloseAction(id codec.ActionId) Option {
	return func(s *Manager) {
		s.closeAction = id
	}
}
func RtHandler(handler RemoteHandler) Option {
	return func(s *Manager) {
		s.remoteHandler = handler
	}
}
func Gateway(host url.Host) Option {
	return func(s *Manager) {
		s.gateway = host
	}
}

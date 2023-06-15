package action

import (
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/socketgateway/service/codec"
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
func DbdProvider(provider codec.DataBuilderProvider) Option {
	return func(s *Manager) {
		s.dateBuilderProvider = provider
	}
}

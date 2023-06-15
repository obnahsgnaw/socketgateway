package action

import (
	"errors"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/codec"
	gatewayv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/gateway/v1"
	"github.com/panjf2000/gnet/v2/pkg/pool/goroutine"
	"sync"
)

type Manager struct {
	handlers            sync.Map // action-id, action-handler
	actions             sync.Map // action-id, [server-host]action-name
	servers             sync.Map // server-host => [action-id]action-name
	closeAction         codec.ActionId
	remoteHandler       RemoteHandler
	dateBuilderProvider codec.DataBuilderProvider // for remote
	gateway             url.Host
}

type actionHandler struct {
	Action  codec.Action
	Handler Handler
}

type Handler func(c socket.Conn, builder codec.DataBuilder, bytes []byte) (respAction codec.Action, respData []byte)

type RemoteHandler interface {
	Call(serverHost, gateway, format string, c socket.Conn, id codec.ActionId, data []byte) (respAction codec.Action, respData []byte, err error)
}

type serverSet map[string]string // [server]action-name

type actionSet map[codec.ActionId]string // [action-id]action-name

func NewManager(options ...Option) *Manager {
	m := &Manager{
		dateBuilderProvider: codec.NewDbp(),
		gateway:             url.Host{},
	}
	m.With(options...)
	return m
}

func (m *Manager) With(options ...Option) {
	for _, o := range options {
		o(m)
	}
}

// HandleClose handle the close action
func (m *Manager) HandleClose(c socket.Conn) {
	pool := goroutine.Default()
	defer pool.Release()
	if h, ok := m.getHandler(m.closeAction); ok {
		h(c, nil, nil)
	}
	for _, s := range m.getServers(m.closeAction) {
		if m.remoteHandler != nil {
			_ = pool.Submit(func() {
				_, _, _ = m.remoteHandler.Call(s, m.gateway.String(), "", c, m.closeAction, nil)
			})
		}
	}
}

func (m *Manager) HandleGatewayErrorResponse(name codec.Name, response *gatewayv1.GatewayErrResponse) ([]byte, error) {
	return m.dateBuilderProvider.Provider(name).Pack(response)
}

// RegisterHandlerAction register an action with handler
func (m *Manager) RegisterHandlerAction(action codec.Action, handler Handler) {
	m.handlers.Store(action.Id, actionHandler{Action: action, Handler: handler})
}

// RegisterRemoteAction register an action with server host
func (m *Manager) RegisterRemoteAction(action codec.Action, serverHost string) {
	var servers serverSet
	var actions actionSet
	if servers1, ok := m.actions.Load(action.Id); ok {
		servers = servers1.(serverSet)
	} else {
		servers = make(serverSet)
	}
	servers[serverHost] = action.Name
	m.actions.Store(action.Id, servers)

	if actions1, ok := m.servers.Load(serverHost); ok {
		actions = actions1.(actionSet)
	} else {
		actions = make(actionSet)
	}
	actions[action.Id] = action.Name
	m.servers.Store(serverHost, actions)

	return
}

// UnregisterRemoteAction unregister a server action
func (m *Manager) UnregisterRemoteAction(serverHost string) {
	if actions, ok := m.servers.Load(serverHost); ok {
		for aid := range actions.(actionSet) {
			if servers, ok := m.actions.Load(aid); ok {
				servers1 := servers.(serverSet)
				delete(servers1, serverHost)
				if len(servers1) > 0 {
					m.actions.Store(aid, servers1)
				} else {
					m.actions.Delete(aid)
				}
			}
		}
		m.servers.Delete(serverHost)
	}
}

func (m *Manager) getServers(actionId codec.ActionId) (list []string) {
	if servers, ok := m.actions.Load(actionId); ok {
		servers1 := servers.(serverSet)
		for s := range servers1 {
			list = append(list, s)
		}
	}
	return
}

func (m *Manager) getRandServer(actionId codec.ActionId) string {
	list := m.getServers(actionId)

	if len(list) == 0 {
		return ""
	}
	return list[utils.RandInt(len(list))]
}

func (m *Manager) getHandler(actionId codec.ActionId) (Handler, bool) {
	if h, ok := m.handlers.Load(actionId); ok {
		return h.(actionHandler).Handler, true
	}

	return nil, false
}

func (m *Manager) GetAction(actionId codec.ActionId) (codec.Action, bool) {
	if actionId == 0 {
		return codec.Action{}, false
	}
	if h, ok := m.handlers.Load(actionId); ok {
		return h.(actionHandler).Action, true
	}
	if h, ok := m.actions.Load(actionId); ok {
		servers := h.(serverSet)
		if len(servers) == 0 {
			m.actions.Delete(actionId)
		} else {
			for _, name := range servers {
				return codec.NewAction(actionId, name), true
			}
		}
	}

	return codec.Action{}, false
}

// Dispatch the actions
func (m *Manager) Dispatch(c socket.Conn, name codec.Name, actionId codec.ActionId, actionData []byte) (respAction codec.Action, respData []byte, err error) {
	if actHandler1, ok := m.handlers.Load(actionId); ok {
		actHandler := actHandler1.(actionHandler)
		respAction, respData = actHandler.Handler(c, m.dateBuilderProvider.Provider(name), actionData)
		return
	}

	s := m.getRandServer(actionId)
	if s == "" {
		err = errors.New("no action handler")
		return
	}
	if m.remoteHandler == nil {
		err = errors.New("no set remote action handler")
		return
	}
	respAction, respData, err = m.remoteHandler.Call(s, m.gateway.String(), name.String(), c, actionId, actionData)
	return
}

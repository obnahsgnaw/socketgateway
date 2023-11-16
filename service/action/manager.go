package action

import (
	"errors"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketutil/codec"
	"github.com/panjf2000/gnet/v2/pkg/pool/goroutine"
	"sort"
	"strings"
	"sync"
)

type Manager struct {
	handlers      sync.Map // action-id, action-handler
	actions       sync.Map // action-id, [server-host]action-name
	servers       sync.Map // server-host => [action-id]action-name
	closeAction   codec.ActionId
	remoteHandler RemoteHandler
	gateway       url.Host
}

type actionHandler struct {
	Action    codec.Action
	structure DataStructure
	Handler   Handler
}

type DataStructure func() codec.DataPtr

type Handler func(c socket.Conn, data codec.DataPtr) (respAction codec.Action, respData codec.DataPtr)

type RemoteHandler interface {
	Call(serverHost, gateway, format string, c socket.Conn, id codec.ActionId, data []byte) (respAction codec.Action, respData []byte, err error)
}

type serverSet map[string][2]string // [server]action-name

type actionSet map[codec.ActionId]string // [action-id]action-name

func NewManager(options ...Option) *Manager {
	m := &Manager{
		gateway: url.Host{},
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
	if _, _, h, ok := m.getHandler(m.closeAction); ok {
		h(c, nil)
	}
	for _, s := range m.getServers(m.closeAction) {
		if m.remoteHandler != nil {
			_ = pool.Submit(func() {
				_, _, _ = m.remoteHandler.Call(s, m.gateway.String(), "", c, m.closeAction, nil)
			})
		}
	}
}

// RegisterHandlerAction register an action with handler
func (m *Manager) RegisterHandlerAction(action codec.Action, structure DataStructure, handler Handler) {
	m.handlers.Store(action.Id, actionHandler{Action: action, structure: structure, Handler: handler})
}

// RegisterRemoteAction register an action with server host
func (m *Manager) RegisterRemoteAction(action codec.Action, serverHost string, flbNum string) {
	var servers serverSet
	var actions actionSet
	if servers1, ok := m.actions.Load(action.Id); ok {
		servers = servers1.(serverSet)
	} else {
		servers = make(serverSet)
	}
	servers[serverHost] = [2]string{
		action.Name,
		flbNum,
	}
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

func (m *Manager) getFlbServers(actionId codec.ActionId) (list map[string]string) {
	list = make(map[string]string)
	if servers, ok := m.actions.Load(actionId); ok {
		servers1 := servers.(serverSet)
		for s, v := range servers1 {
			if v[1] != "" {
				hp := strings.Split(s, ":")
				list[hp[0]+":"+v[1]] = s
			} else {
				list[s] = s
			}
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

func (m *Manager) getFlbServer(fd int, actionId codec.ActionId) string {
	list := m.getFlbServers(actionId)
	if len(list) == 0 {
		return ""
	}
	var flbK []string
	for k := range list {
		flbK = append(flbK, k)
	}
	sort.Strings(flbK)

	if fd <= 0 {
		return list[flbK[0]]
	}

	index := fd % len(flbK)
	return list[flbK[index]]
}

func (m *Manager) getHandler(actionId codec.ActionId) (codec.Action, DataStructure, Handler, bool) {
	if h, ok := m.handlers.Load(actionId); ok {
		h1 := h.(actionHandler)
		return h1.Action, h1.structure, h1.Handler, true
	}

	return codec.Action{}, nil, nil, false
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
				return codec.NewAction(actionId, name[0]), true
			}
		}
	}

	return codec.Action{}, false
}

// Dispatch the actions
func (m *Manager) Dispatch(c socket.Conn, name codec.Name, b codec.DataBuilder, actionId codec.ActionId, actionData []byte) (respAction codec.Action, respData []byte, err error) {
	if _, s, h, ok := m.getHandler(actionId); ok {
		p := s()
		if err = b.Unpack(actionData, p); err != nil {
			return codec.Action{}, nil, err
		}
		var respPtr codec.DataPtr
		respAction, respPtr = h(c, p)
		respData, err = b.Pack(respPtr)
		return
	}

	flbNum := c.Fd()
	// 可自定义flb-action.string：xxx 来知道conn对某个action的负载均衡策略
	if v, ok := c.Context().GetOptional("flb-" + actionId.String()); ok {
		if vv, ok := v.(int); ok {
			flbNum = vv
		}
	}
	s := m.getFlbServer(flbNum, actionId)
	if s == "" {
		err = errors.New("action manager error: no action handler")
		return
	}
	if m.remoteHandler == nil {
		err = errors.New("action manager error: no set remote action handler")
		return
	}
	respAction, respData, err = m.remoteHandler.Call(s, m.gateway.String(), name.String(), c, actionId, actionData)
	return
}

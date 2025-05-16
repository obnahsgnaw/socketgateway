package action

import (
	"errors"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/pkg/utils"
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler/connutil"
	"github.com/obnahsgnaw/socketutil/codec"
	"github.com/panjf2000/gnet/v2/pkg/pool/goroutine"
	"sort"
	"strings"
	"sync"
)

type Manager struct {
	handlers            sync.Map // action-id, action-handler
	actions             sync.Map // action-id, [server-host]action-name
	servers             sync.Map // server-host => [action-id]action-name
	closeAction         codec.ActionId
	authenticateActions sync.Map // map[authenticateType]codec.ActionId
	rawActions          sync.Map // map[protocolType]codec.ActionId
	remoteHandler       RemoteHandler
	gateway             url.Host
}

type actionHandler struct {
	Action    codec.Action
	structure DataStructure
	Handler   Handler
}

type DataStructure func() codec.DataPtr

type Handler func(c socket.Conn, data codec.DataPtr) (respAction codec.Action, respData codec.DataPtr)

type RemoteHandler interface {
	Call(rqId, serverHost, gateway, format string, c socket.Conn, id codec.ActionId, data []byte) (respAction codec.Action, respData []byte, err error)
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
	if _, _, h, ok := m.getHandler(m.closeAction); ok {
		h(c, nil)
	}
	pool := goroutine.Default()
	defer pool.Release()
	var wg sync.WaitGroup
	for _, s := range m.getServers(m.closeAction) {
		if m.remoteHandler != nil {
			wg.Add(1)
			_ = pool.Submit(m.closeTask(c, s, &wg))
		}
	}
	wg.Wait()
}

func (m *Manager) closeTask(c socket.Conn, gw string, wg *sync.WaitGroup) func() {
	return func() {
		_, _, _ = m.remoteHandler.Call("", gw, m.gateway.String(), "", c, m.closeAction, nil)
		wg.Done()
	}
}

// RegisterHandlerAction register an action with handler
func (m *Manager) RegisterHandlerAction(action codec.Action, structure DataStructure, handler Handler) {
	m.handlers.Store(action.Id, actionHandler{Action: action, structure: structure, Handler: handler})
	m.initType(action)
}

// RegisterRemoteAction register an action with server host
// action.Name authenticate:xxx,raw:xxx for customer action
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
	m.initType(action)

	return
}

func (m *Manager) initType(action codec.Action) {
	if strings.Contains(action.Name, ":") {
		valSegments := strings.Split(action.Name, ":")
		switch valSegments[0] {
		case "authenticate":
			if strings.Contains(valSegments[1], ",") {
				tps := strings.Split(valSegments[1], ",")
				for _, tp := range tps {
					m.authenticateActions.Store(tp, action.Id)
				}
			} else {
				m.authenticateActions.Store(valSegments[1], action.Id)
			}
			break
		case "raw":
			if strings.Contains(valSegments[1], ",") {
				tps := strings.Split(valSegments[1], ",")
				for _, tp := range tps {
					m.rawActions.Store(tp, action.Id)
				}
			} else {
				m.rawActions.Store(valSegments[1], action.Id)
			}
			break
		default:
		}
	}
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
		return list[flbK[utils.RandInt(len(flbK))]]
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
func (m *Manager) Dispatch(c socket.Conn, rqId string, b codec.DataBuilder, actionId codec.ActionId, actionData []byte) (respAction codec.Action, respData []byte, err error) {
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
	if v := connutil.GetActionFlb(c, actionId); v > 0 {
		flbNum = v
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
	respAction, respData, err = m.remoteHandler.Call(rqId, s, m.gateway.String(), b.Name().String(), c, actionId, actionData)
	return
}

func (m *Manager) Authenticate(c socket.Conn, rqId string, b codec.DataBuilder, tp, id string, secret string) (auth *socket.Authentication, key []byte, err error) {
	rq := &handlerv1.AuthenticateRequest{
		Gateway: m.gateway.String(),
		Fd:      int64(c.Fd()),
		Type:    tp,
		Id:      id,
		Secret:  secret,
	}
	var bb []byte
	if bb, err = b.Pack(rq); err != nil {
		return
	}
	aid, ok := m.authenticateActions.Load(tp)
	if !ok {
		err = errors.New("authenticate type not support")
		return
	}
	if _, bb, err = m.Dispatch(c, rqId, b, aid.(codec.ActionId), bb); err != nil {
		return
	}
	var response handlerv1.AuthenticateResponse
	if err = b.Unpack(bb, &response); err != nil {
		return
	}
	if response.Error != "" {
		if response.Error == "NO_CERT" {
			response.Key = []byte("NO_CERT")
		} else {
			err = errors.New(response.Error)
			return
		}
	} else {
		if string(response.Key) == "NO_CERT" {
			response.Key = nil
		}
	}
	auth = &socket.Authentication{
		Type:     response.Type,
		Id:       response.Id,
		Iid:      response.Iid,
		Sn:       response.Sn,
		Cid:      response.CompanyId,
		Uid:      response.UserId,
		Protocol: response.Protocol,
		Config:   response.Config,
	}
	key = response.Key
	return
}

func (m *Manager) Raw(c socket.Conn, rqId string, b codec.DataBuilder, tp string, actionData []byte, actionId uint32) (respData []byte, subActions []*handlerv1.SubAction, err error) {
	// encode to raw request
	rawRequest := &handlerv1.RawRequest{ActionId: actionId, Data: actionData}
	var rawByte []byte
	var respAction codec.Action
	var response handlerv1.RawResponse
	if rawByte, err = b.Pack(rawRequest); err != nil {
		return
	}

	// find the raw handler
	rawActId, ok := m.rawActions.Load(tp)
	if !ok {
		err = errors.New("raw type no handler")
		return
	}
	// when the action id is 0, to raw handler to handle input.
	if actionId == 0 {
		// handle raw action
		if respAction, rawByte, err = m.Dispatch(c, rqId, b, rawActId.(codec.ActionId), rawByte); err != nil {
			return
		}
		if err = b.Unpack(rawByte, &response); err != nil {
			return
		}
		// when the raw input handle response action not 0, dispatch to the action handler to handle
		subActions = response.SubActions
		if respAction.Id > 0 {
			// dispatch to action handler
			if respAction, rawByte, err = m.Dispatch(c, rqId, b, respAction.Id, response.Data); err != nil {
				return
			}
			// when action handle response a action, dispatch to raw handler to trans output
			if respAction.Id > 0 {
				respData, _, err = m.Raw(c, rqId, b, tp, rawByte, uint32(respAction.Id))
			}
		}
		return
	}
	// otherwise to handle the output
	if _, rawByte, err = m.Dispatch(c, rqId, b, rawActId.(codec.ActionId), rawByte); err != nil {
		return
	}
	if err = b.Unpack(rawByte, &response); err != nil {
		return
	}
	respData = response.Data
	return
}

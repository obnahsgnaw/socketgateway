package socketgateway

import (
	"errors"
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/application/regtype"
	"github.com/obnahsgnaw/application/servertype"
	"github.com/obnahsgnaw/application/service/regCenter"
	rpc2 "github.com/obnahsgnaw/rpc"
	"github.com/obnahsgnaw/socketgateway/asset"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/gnet"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/net"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/action"
	"github.com/obnahsgnaw/socketgateway/service/codec"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	bindv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/bind/v1"
	gatewayv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/gateway/v1"
	groupv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/group/v1"
	messagev1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/message/v1"
	"github.com/obnahsgnaw/socketgateway/service/proto/impl"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"strconv"
	"strings"
	"time"
)

// socket, rpc, doc, 注册socket-gateway、发现socket-gateway、发现socket-server

type Server struct {
	id             string // 模块-子模块
	name           string
	st             servertype.ServerType
	sct            sockettype.SocketType
	et             endtype.EndType
	host           url.Host
	dc             *DocConfig
	app            *application.Application
	m              *action.Manager
	ss             *socket.Server
	e              *eventhandler.Event
	regInfo        *regCenter.RegInfo
	rs             *rpc2.Server
	ds             *DocServer
	logger         *zap.Logger
	handlerRegInfo *regCenter.RegInfo
	err            error
	poll           bool
	regEnable      bool
	keepalive      uint
	tickInterval   time.Duration
}

func sct2st(st sockettype.SocketType) (sst servertype.ServerType) {
	switch st {
	case sockettype.TCP, sockettype.TCP4, sockettype.TCP6:
		sst = servertype.Tcp
		break
	case sockettype.WSS:
		sst = servertype.Wss
		break
	case sockettype.UDP, sockettype.UDP4, sockettype.UDP6:
		sst = servertype.Udp
		break
	default:
		sst = ""
	}
	return
}

func New(app *application.Application, st sockettype.SocketType, et endtype.EndType, host url.Host, options ...Option) *Server {
	s := &Server{
		id:   "gateway-gateway",
		name: "gateway",
		st:   sct2st(st),
		sct:  st,
		et:   et,
		host: host,
		app:  app,
		m:    action.NewManager(),
	}
	s.logger, s.err = logger.New(utils.ToStr("Soc[", et.String(), "-", st.String(), "-gateway]"), app.LogConfig(), app.Debugger().Debug())
	s.e = eventhandler.New(app.Context(), s.m, st, eventhandler.Logger(s.logger))
	s.regInfo = &regCenter.RegInfo{
		AppId:   s.app.ID(),
		RegType: regtype.RegType(s.st),
		ServerInfo: regCenter.ServerInfo{
			Id:      s.id,
			Name:    s.name,
			Type:    s.st.String(),
			EndType: s.et.String(),
		},
		Host: s.host.String(),
		Val:  s.host.String(),
		Ttl:  s.app.RegTtl(),
	}
	s.handlerRegInfo = &regCenter.RegInfo{
		AppId:   s.app.ID(),
		RegType: regtype.Rpc,
		ServerInfo: regCenter.ServerInfo{
			Id:      s.id,
			Name:    s.name,
			Type:    servertype.Hdl.String(),
			EndType: s.et.String(),
		},
		Host:      "",
		Val:       "",
		Ttl:       s.app.RegTtl(),
		KeyPreGen: regCenter.ActionRegKeyPrefixGenerator(),
	}
	s.With(options...)
	return s
}

func (s *Server) With(options ...Option) {
	for _, o := range options {
		o(s)
	}
}

// ID return the api service id
func (s *Server) ID() string {
	return s.id
}

// Name return the api service name
func (s *Server) Name() string {
	return s.name
}

// Type return the api server end type
func (s *Server) Type() servertype.ServerType {
	return s.st
}

// EndType return the api server end type
func (s *Server) EndType() endtype.EndType {
	return s.et
}

// RegEnabled reg http
func (s *Server) RegEnabled() bool {
	return s.regEnable
}

// RegInfo return the server register info
func (s *Server) RegInfo() *regCenter.RegInfo {
	return s.regInfo
}

// Logger return the logger
func (s *Server) Logger() *zap.Logger {
	return s.logger
}

// WithRpcServer new rpc server for socket
func (s *Server) WithRpcServer(port int) *rpc2.Server {
	ss := rpc2.New(s.app, s.sct.String()+"-gateway", utils.ToStr(s.sct.String(), "-", s.id, "-rpc"), s.et, url.Host{
		Ip:   s.host.Ip,
		Port: port,
	}, rpc2.RegEnable(), rpc2.Parent(s))
	ss.RegisterService(rpc2.ServiceInfo{
		Desc: bindv1.BindService_ServiceDesc,
		Impl: impl.NewBindService(s.ss),
	})
	ss.RegisterService(rpc2.ServiceInfo{
		Desc: groupv1.GroupService_ServiceDesc,
		Impl: impl.NewGroupService(s.ss, s.e),
	})
	ss.RegisterService(rpc2.ServiceInfo{
		Desc: messagev1.MessageService_ServiceDesc,
		Impl: impl.NewMessageService(s.ss, s.e),
	})
	s.m.With(action.Gateway(ss.Host()))
	s.m.With(action.RtHandler(impl.NewRemoteHandler()))
	s.rs = ss
	s.debug("withed gateway rpc server")

	return ss
}

func (s *Server) WithDocServer(gwPrefix string, port int) {
	config := &DocConfig{
		id:      s.id,
		endType: s.et,
		Origin: url.Origin{
			Protocol: url.HTTP,
			Host: url.Host{
				Ip:   s.host.Ip,
				Port: port,
			},
		},
		RegTtl:        s.app.RegTtl(),
		gwPrefix:      gwPrefix,
		socketGateway: true,
		Doc: DocItem{
			socketType: s.sct,
			Path:       "/doc",
			Title:      s.name,
			Provider: func() ([]byte, error) {
				return asset.Asset("service/doc/html/gateway.html")
			},
		},
	}
	s.ds = NewDocServer(s.app.ID(), config)
	s.debug("withed gateway doc server")
}

func (s *Server) WatchLog(watcher func(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field)) {
	s.e.WatchLog(watcher)
}

func (s *Server) Engine() *socket.Server {
	return s.ss
}

func (s *Server) AddTicker(ticker eventhandler.TickHandler) {
	s.e.AddTicker(ticker)
}

func (s *Server) Listen(action codec.Action, structure action.DataStructure, handler action.Handler) {
	s.m.RegisterHandlerAction(action, structure, handler)
}

func (s *Server) Send(c socket.Conn, id codec.Action, data codec.DataPtr) (err error) {
	return s.e.SendAction(c, id, data)
}

func (s *Server) Rpc() *rpc2.Server {
	return s.rs
}

func (s *Server) Release() {
	if s.RegEnabled() {
		s.debug("unregister soc")
		_ = s.app.DoUnregister(s.regInfo)
	}
	s.debug("unregister doc")
	_ = s.app.DoUnregister(s.ds.regInfo)
	if s.rs != nil {
		s.debug("release rpc server")
		s.rs.Release()
	}
	s.debug("release logger")
	_ = s.logger.Sync()
}

func (s *Server) Run(failedCb func(error)) {
	if s.st == "" {
		failedCb(errors.New(s.msg("type not support")))
		return
	}
	if s.err != nil {
		failedCb(s.err)
		return
	}
	var e socket.Engine
	if s.poll {
		e = gnet.New() // TODO windows问题
	} else {
		e = net.New()
	}
	s.ss = socket.New(s.app.Context(), s.sct, s.host.Port, e, s.e, &socket.Config{
		MultiCore: true,
		Keepalive: s.keepalive,
		NoDelay:   true,
		Ticker:    s.tickInterval > 0,
	})
	if s.app.Register() != nil {
		if s.RegEnabled() {
			if err := s.app.DoRegister(s.regInfo); err != nil {
				failedCb(utils.NewWrappedError(s.msg("register soc failed"), err))
			}
		}
		if err := s.app.DoRegister(s.ds.regInfo); err != nil {
			failedCb(utils.NewWrappedError(s.msg("register doc failed"), err))
		}
		if err := s.watch(s.app.Register()); err != nil {
			failedCb(utils.NewWrappedError(s.msg("watch failed"), err))
		}
	}
	s.Listen(action.New(gatewayv1.ActionId_Ping),
		func() codec.DataPtr {
			return &gatewayv1.PingRequest{}
		},
		func(c socket.Conn, data codec.DataPtr) (respAction codec.Action, respData codec.DataPtr) {
			respAction = action.New(gatewayv1.ActionId_Pong)
			return
		},
	)
	s.logger.Info(utils.ToStr(s.st.String(), "[", s.host.String(), "] start and serving..."))
	s.ss.SyncStart(failedCb)
	if s.ds != nil {
		docDesc := utils.ToStr("doc[", s.ds.config.Origin.Host.String(), "] ")
		s.logger.Info(docDesc + "start and serving...")
		s.debug("doc url=" + s.ds.DocUrl())
		s.debug("index doc url=" + s.ds.IndexDocUrl())
		if s.ds.config.gwPrefix != "" {
			s.debug("gateway doc format=" + s.ds.GatewayDocUrlFormat())
		}
		s.ds.SyncStart(failedCb)
	}
	if s.rs != nil {
		s.rs.Run(failedCb)
	}
}

func (s *Server) msg(msg ...string) string {
	return utils.ToStr("Socket Server[", s.name, "] ", utils.ToStr(msg...))
}

func (s *Server) watch(register regCenter.Register) error {
	if s.regEnable {
		// watch socket
	}
	if s.ds != nil {
		prefix := s.ds.regInfo.Prefix()
		if prefix == "" {
			return errors.New("watch key prefix is empty")
		}
		err := register.Watch(s.app.Context(), prefix, func(key string, val string, isDel bool) {
			segments := strings.Split(key, "/")
			id := segments[len(segments)-3]
			idSegments := strings.Split(id, "-")
			moduleName := idSegments[0]
			keyName := idSegments[1]
			attr := segments[len(segments)-1]
			if isDel {
				if attr == "url" {
					s.debug(utils.ToStr("sub doc[", moduleName, ":", keyName, "] leaved"))
					s.ds.Manager.Remove(moduleName, keyName, val)
				}
			} else {
				if attr == "url" {
					s.debug(utils.ToStr("sub doc[", moduleName, ":", keyName, "] joined"))
					s.ds.Manager.Add(moduleName, keyName, "", val)
				}
				if attr == "title" {
					s.ds.Manager.Add(moduleName, keyName, val, "")
				}
			}
		})
		if err != nil {
			return err
		}
	}
	// watch handler
	handlerPrefix := s.handlerRegInfo.Prefix()
	err := register.Watch(s.app.Context(), handlerPrefix, func(key string, val string, isDel bool) {
		segments := strings.Split(key, "/")
		id := segments[len(segments)-3]
		host := segments[len(segments)-2]
		actionId, _ := strconv.Atoi(segments[len(segments)-1])
		idSegments := strings.Split(id, "-")
		moduleName := idSegments[0]
		keyName := idSegments[1]
		if isDel {
			s.debug(utils.ToStr("action [", moduleName, ":", keyName, "]", strconv.Itoa(actionId), " leaved"))
			s.m.UnregisterRemoteAction(host)
		} else {
			s.debug(utils.ToStr("action [", moduleName, ":", keyName, "]", strconv.Itoa(actionId), " added"))
			s.m.RegisterRemoteAction(codec.Action{
				Id:   codec.ActionId(actionId),
				Name: val,
			}, host)
		}
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) debug(msg string) {
	if s.app.Debugger().Debug() {
		s.logger.Debug(msg)
	}
}

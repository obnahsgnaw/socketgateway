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
	"github.com/obnahsgnaw/goutils/randutil"
	rpc2 "github.com/obnahsgnaw/rpc"
	"github.com/obnahsgnaw/rpc/pkg/rpcclient"
	bindv1 "github.com/obnahsgnaw/socketapi/gen/bind/v1"
	connv1 "github.com/obnahsgnaw/socketapi/gen/conninfo/v1"
	groupv1 "github.com/obnahsgnaw/socketapi/gen/group/v1"
	messagev1 "github.com/obnahsgnaw/socketapi/gen/message/v1"
	slbv1 "github.com/obnahsgnaw/socketapi/gen/slb/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/mqtt"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/custom/http"
	mqtt2 "github.com/obnahsgnaw/socketgateway/pkg/socket/engine/custom/mqtt"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/action"
	"github.com/obnahsgnaw/socketgateway/service/doc"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	"github.com/obnahsgnaw/socketgateway/service/manage"
	gatewayv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/gateway/v1"
	"github.com/obnahsgnaw/socketgateway/service/proto/impl"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	"strconv"
	"strings"
	"time"
)

const closeAction = 0

// socket-tcp,websocket, rpc-rpc, doc-api, 注册socket-gateway、发现socket-gateway、发现socket-server
// socket port === required
// rpc    port === optional，enabled to discover handler and sub doc
// api    port === optional, for doc view page

type Server struct {
	app             *application.Application
	id              string // 模块-子模块
	name            string
	endType         endtype.EndType
	rawServerType   servertype.ServerType
	rawSocketType   sockettype.SocketType
	businessChannel string
	host            url.Host
	server          *socket.Server
	engine          socket.Engine
	rpcServer       *rpc2.Server
	actManager      *action.Manager
	actListeners    []func(*action.Manager)
	eventHandler    *eventhandler.Event
	docServer       *DocServer
	eo              []eventhandler.Option
	logger          *zap.Logger
	logCnf          *logger.Config
	regInfo         *regCenter.RegInfo
	watchHdlRegInfo *regCenter.RegInfo
	errs            []error
	poll            bool
	regEnable       bool
	reuseAddr       bool
	keepalive       uint
	tickInterval    time.Duration
	authProvider    AuthProvider
	running         bool
	watchGwRegInfo  *regCenter.RegInfo // watch gateway
	watchClient     *rpcclient.Manager
	manager         *manage.Manager
	managerTrigger  *manage.Trigger

	mqttAddr        string
	mqttOptions     []mqtt.Option
	mqttRawTopics   []mqtt2.QosTopic
	mqttClientTopic *mqtt2.QosTopic
	mqttServerTopic *mqtt2.QosTopic
}

type AuthProvider interface {
	GetAuthedUser(token string) (*socket.AuthUser, error)
	GetIdUser(uid string) (*socket.AuthUser, error)
}

func New(app *application.Application, st sockettype.SocketType, et endtype.EndType, host url.Host, businessChannel string, options ...Option) *Server {
	var err error
	s := &Server{
		id:              "gateway-gateway",
		name:            "gateway",
		rawServerType:   st.ToServerType(),
		rawSocketType:   st,
		businessChannel: businessChannel,
		endType:         et,
		host:            host,
		app:             app,
		actManager:      action.NewManager(),
		watchClient:     rpcclient.NewManager(),
	}
	if s.rawServerType == "" {
		s.addErr(errors.New(s.msg("type not support")))
	}
	s.logCnf = s.app.LogConfig()
	s.logger = s.app.Logger().Named(utils.ToStr(s.businessChannel, "-", s.rawServerType.String(), "-", s.endType.String(), "-", "gateway"))
	s.addErr(err)
	s.With(options...)
	s.initRegInfo()
	s.manager = manage.New(s.rawServerType.String(), s.server, &manage.GwConfig{ // TODO
		Port:              0,
		RpcPort:           0,
		DocDisable:        false,
		KeepaliveInterval: 0,
		AuthCheckInterval: 0,
		HeartbeatInterval: 0,
		TickInterval:      0,
		Security:          false,
		SecurityType:      "",
		SecurityEncoder:   "",
		SecurityEncode:    false,
	}, func(c socket.Conn, act codec.Action, pkg []byte) error {
		return s.eventHandler.Send(c, "", act, pkg)
	})
	s.managerTrigger = manage.NewTrigger(s.manager)

	return s
}

func (s *Server) initRegInfo() {
	s.regInfo = &regCenter.RegInfo{
		AppId:   s.app.Cluster().Id(),
		RegType: "socket-gw",
		ServerInfo: regCenter.ServerInfo{
			Id:      s.id,
			Name:    s.name,
			Type:    s.businessChannel,
			EndType: s.endType.String(),
		},
		Host: s.host.String(),
		Val:  s.host.String(),
		Ttl:  s.app.RegTtl(),
	}
	s.watchHdlRegInfo = &regCenter.RegInfo{
		AppId:   s.app.Cluster().Id(),
		RegType: regtype.Rpc,
		ServerInfo: regCenter.ServerInfo{
			Type:    "socket-hdl@" + s.businessChannel,
			EndType: s.endType.String(),
		},
		Ttl:       s.app.RegTtl(),
		KeyPreGen: regCenter.ActionRegKeyPrefixGenerator(),
	}
	s.watchGwRegInfo = &regCenter.RegInfo{
		AppId:   s.app.Cluster().Id(),
		RegType: regtype.Rpc,
		ServerInfo: regCenter.ServerInfo{
			Type:    "socket-gw@" + s.businessChannel,
			EndType: s.endType.String(),
		},
	}
}

func (s *Server) With(options ...Option) {
	for _, o := range options {
		o(s)
	}
}

// ID return the service id
func (s *Server) ID() string {
	return s.id
}

// Name return the service name
func (s *Server) Name() string {
	return s.name
}

// Type return the server end type
func (s *Server) Type() servertype.ServerType {
	return s.rawServerType
}

// EndType return the server end type
func (s *Server) EndType() endtype.EndType {
	return s.endType
}

func (s *Server) Release() {
	var cb = func(msg string) {
		s.logger.Debug(msg)
	}
	if s.RegEnabled() && s.app.Register() != nil {
		_ = s.app.DoUnregister(s.regInfo, cb)
	}
	if s.docServer != nil && s.app.Register() != nil {
		_ = s.app.DoUnregister(s.docServer.regInfo, cb)
	}
	if s.rpcServer != nil {
		s.rpcServer.Release()
	}
	s.logger.Info("released")
	_ = s.logger.Sync()
	s.running = false
}

func (s *Server) Run(failedCb func(error)) {
	if s.running {
		return
	}
	if s.errs != nil {
		failedCb(s.errs[0])
		return
	}
	s.logger.Info("init start...")
	if s.engine == nil {
		if s.rawSocketType == sockettype.HTTP {
			s.engine = http.New(s.host.Ip)
		} else if s.rawSocketType == sockettype.MQTT {
			if s.mqttAddr == "" {
				failedCb(errors.New(s.msg("mqtt addr not set")))
				return
			}
			eg := mqtt2.New(s.mqttAddr, s.mqttOptions...)
			if s.mqttServerTopic != nil && s.mqttClientTopic != nil {
				if err := eg.ListenTopic(*s.mqttClientTopic, *s.mqttServerTopic); err != nil {
					failedCb(errors.New(s.msg(err.Error())))
					return
				}
			}
			if err := eg.ListenRawTopic(s.mqttRawTopics...); err != nil {
				failedCb(errors.New(s.msg(err.Error())))
				return
			}
			s.engine = eg
		} else {
			s.engine = engine.Default()
		}
		s.logger.Info("socket engine initialize(default)")
	} else {
		s.logger.Info("socket engine initialize(customer)")
	}
	s.addEventOption(eventhandler.Logger(s.logger))
	s.addEventOption(eventhandler.Manage(s.managerTrigger))
	s.eventHandler = eventhandler.New(s.app.Context(), s.actManager, s.rawSocketType, s.eo...)
	s.server = socket.New(s.app.Context(), s.rawSocketType, s.host.Port, s.engine, s.eventHandler, &socket.Config{
		MultiCore: true,
		Keepalive: s.keepalive,
		NoDelay:   true,
		Ticker:    s.tickInterval > 0,
		ReuseAddr: s.reuseAddr,
	}, s.watchClient)
	s.logger.Info("socket server initialized")
	s.defaultListen()
	for _, h := range s.actListeners {
		h(s.actManager)
	}
	s.logger.Info("listen action initialized")
	regLogCb := func(msg string) {
		s.logger.Debug(msg)
	}
	if s.app.Register() != nil {
		if s.RegEnabled() {
			s.logger.Debug("server register start...")
			if err := s.app.DoRegister(s.regInfo, regLogCb); err != nil {
				failedCb(s.socketGwError(s.msg("register soc failed"), err))
			}
			s.logger.Debug("server registered")
		}
		if err := s.watch(s.app.Register()); err != nil {
			failedCb(s.socketGwError(s.msg("watch failed"), err))
		}
	}
	if s.docServer != nil {
		s.logger.Info("doc server enabled, init start...")
		s.logger.Info(utils.ToStr("doc index url=", s.docServer.IndexDocUrl(), ", doc url=", s.docServer.DocUrl()))
		docDesc := utils.ToStr("doc server[", s.docServer.engine.Host(), "] ")
		s.logger.Info(docDesc + "start and serving...")
		s.docServer.SyncStart(randutil.RandAlpha(6), failedCb)
		if s.app.Register() != nil {
			s.logger.Debug("doc register start")
			if err := s.app.DoRegister(s.docServer.regInfo, regLogCb); err != nil {
				failedCb(s.socketGwError(s.msg("register doc failed"), err))
			}
			s.logger.Debug("doc registered")
		}
		s.logger.Debug("sub doc watch start...")
		prefix := s.docServer.regInfo.Prefix()
		if prefix == "" {
			failedCb(errors.New("watch key prefix is empty"))
			return
		}
		err := s.app.Register().Watch(s.app.Context(), prefix, func(key string, val string, isDel bool) {
			segments := strings.Split(key, "/")
			id := segments[len(segments)-3]
			idSegments := strings.Split(id, "-")
			moduleName := idSegments[0]
			keyName := idSegments[1]
			attr := segments[len(segments)-1]
			if isDel {
				if attr == "url" {
					s.logger.Debug(utils.ToStr("sub doc[", moduleName, ":", keyName, "] leaved"))
					s.docServer.Manager.Remove(moduleName, keyName, val)
				}
			} else {
				if attr == "url" {
					s.logger.Debug(utils.ToStr("sub doc[", moduleName, ":", keyName, "] joined"))
					s.docServer.Manager.Add(moduleName, keyName, "", val, nil)
				}
				if attr == "title" {
					s.docServer.Manager.Add(moduleName, keyName, val, "", nil)
				}
				if attr == "public" {
					var public bool
					if val == "1" {
						public = true
					}
					s.docServer.Manager.Add(moduleName, keyName, "", "", &public)
				}
			}
		})
		if err != nil {
			failedCb(err)
			return
		}
	}
	if s.rpcServer != nil {
		s.logger.Info("rpc server enabled, init start...")
		s.initRpc()
		s.rpcServer.Run(failedCb)
	}
	s.logger.Info(utils.ToStr("socket[", s.host.String(), "] start and serving..."))
	s.server.SyncStart(failedCb)
	s.running = true
}

func (s *Server) Listen(act codec.Action, structure action.DataStructure, handler action.Handler) {
	s.actListeners = append(s.actListeners, func(manager *action.Manager) {
		manager.RegisterHandlerAction(act, structure, handler)
		s.logger.Debug("listened action:" + act.Name)
	})
}

func (s *Server) Handler() *eventhandler.Event {
	return s.eventHandler
}

func (s *Server) Send(c socket.Conn, act codec.Action, data codec.DataPtr) (err error) {
	return s.eventHandler.SendAction(c, act, data)
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

func (s *Server) LogConfig() *logger.Config {
	return s.logCnf
}

func (s *Server) Engine() *socket.Server {
	return s.server
}

func (s *Server) Rpc() *rpc2.Server {
	return s.rpcServer
}

func (s *Server) DocServer() *DocServer {
	return s.docServer
}

func (s *Server) initRpc() {
	if s.rpcServer != nil {
		s.rpcServer.RegisterService(rpc2.ServiceInfo{
			Desc: bindv1.BindService_ServiceDesc,
			Impl: impl.NewBindService(func() *socket.Server { return s.server }),
		})
		s.rpcServer.RegisterService(rpc2.ServiceInfo{
			Desc: connv1.ConnService_ServiceDesc,
			Impl: impl.NewConnService(func() *socket.Server { return s.server }),
		})
		s.rpcServer.RegisterService(rpc2.ServiceInfo{
			Desc: groupv1.GroupService_ServiceDesc,
			Impl: impl.NewGroupService(func() *socket.Server { return s.server }, func() *eventhandler.Event { return s.eventHandler }),
		})
		s.rpcServer.RegisterService(rpc2.ServiceInfo{
			Desc: messagev1.MessageService_ServiceDesc,
			Impl: impl.NewMessageService(func() *socket.Server { return s.server }, func() *eventhandler.Event { return s.eventHandler }),
		})
		s.rpcServer.RegisterService(rpc2.ServiceInfo{
			Desc: slbv1.SlbService_ServiceDesc,
			Impl: impl.NewSlbService(func() *socket.Server { return s.server }),
		})
		s.actManager.With(action.CloseAction(closeAction))
		s.actManager.With(action.Gateway(s.rpcServer.Host()))
		s.actManager.With(action.RtHandler(impl.NewRemoteHandler(s.app.Context(), s.logger, s.businessChannel)))
	}
}

func (s *Server) docConfig() *DocConfig {
	return &DocConfig{
		id:       s.id,
		endType:  s.endType,
		servType: servertype.ServerType(s.businessChannel),
		RegTtl:   s.app.RegTtl(),
		Doc: DocItem{
			socketType: sockettype.SocketType(s.businessChannel),
			Title:      s.name,
			Public:     true,
			Provider: func() ([]byte, error) {
				return doc.Assets.ReadFile("html/gateway.html")
			},
		},
	}
}

func (s *Server) addEventOption(option eventhandler.Option) {
	s.eo = append(s.eo, option)
}

func (s *Server) socketGwError(msg string, err error) error {
	return utils.TitledError(utils.ToStr("socket gateway[", s.name, "] error"), msg, err)
}

func (s *Server) defaultListen() {
	s.Listen(action.New(gatewayv1.ActionId_Ping),
		func() codec.DataPtr {
			return &gatewayv1.PingRequest{}
		},
		func(c socket.Conn, data codec.DataPtr) (respAction codec.Action, respData codec.DataPtr) {
			respAction = action.New(gatewayv1.ActionId_Pong)
			respData = &gatewayv1.PongResponse{
				Timestamp: timestamppb.New(time.Now()),
			}
			return
		},
	)
	s.Listen(action.New(gatewayv1.ActionId_AuthReq),
		func() codec.DataPtr {
			return &gatewayv1.AuthRequest{}
		},
		func(c socket.Conn, data codec.DataPtr) (respAction codec.Action, respData codec.DataPtr) {
			q := data.(*gatewayv1.AuthRequest)
			respAction = action.New(gatewayv1.ActionId_AuthResp)
			response := &gatewayv1.AuthResponse{
				Success: false,
			}
			respData = response
			if c.Context().Authed() {
				response.Success = true
				return
			}
			if s.authProvider == nil {
				response.Success = true
			} else {
				u, err := s.authProvider.GetAuthedUser(q.Token)
				if err != nil {
					s.Logger().Error(s.msg("auth action request resp error: err=" + err.Error()))
					response.Success = false
				} else {
					response.Success = true
					if u.Attr == nil {
						u.Attr = make(map[string]string)
					}
					uid, _ := u.Attr["user_id"]
					cidStr, _ := u.Attr["company_id"]
					cid, _ := strconv.Atoi(cidStr)
					if uid != c.Context().Authentication().Id {
						s.Logger().Error(s.msg("auth action request resp error: uid diff of authenticate id", uid, c.Context().Authentication().Id))
						response.Success = false
						return
					}
					err = s.server.Authenticate(c, &socket.Authentication{
						Type: c.Context().Authentication().Type,
						Id:   uid,
						Cid:  uint32(cid),
						Uid:  uint32(u.Id),
					})
					if err != nil {
						s.Logger().Error(s.msg("auth action request resp error: err=" + err.Error()))
						response.Success = false
						return
					}
					s.server.Auth(c, u)
				}
			}

			return
		},
	)
}

func (s *Server) msg(msg ...string) string {
	return utils.ToStr("Socket Server[", s.name, "] ", utils.ToStr(msg...))
}

func (s *Server) watch(register regCenter.Register) error {
	if s.regEnable {
		// watch socket
	}
	// watch handler
	s.logger.Debug("handler watch start...")
	handlerPrefix := s.watchHdlRegInfo.Prefix()
	err := register.Watch(s.app.Context(), handlerPrefix, func(key string, val string, isDel bool) {
		segments := strings.Split(key, "/")
		id := segments[len(segments)-3]
		host := segments[len(segments)-2]
		actionStr := segments[len(segments)-1]
		actionId, _ := strconv.Atoi(actionStr)
		idSegments := strings.Split(id, "-")
		moduleName := idSegments[0]
		keyName := idSegments[1]
		if isDel {
			s.logger.Debug(utils.ToStr("action [", moduleName, ":", keyName, "]", strconv.Itoa(actionId), " leaved"))
			s.actManager.UnregisterRemoteAction(host)
		} else {
			s.logger.Debug(utils.ToStr("action [", moduleName, ":", keyName, "]", strconv.Itoa(actionId), " added"))
			flbNum := ""
			if strings.Contains(val, "|") {
				valNum := strings.Split(val, "|")
				val = valNum[0]
				flbNum = valNum[1]
			}
			s.actManager.RegisterRemoteAction(codec.Action{
				Id:   codec.ActionId(actionId),
				Name: val,
			}, host, flbNum)
		}
	})
	if err != nil {
		return err
	}

	// watch gateway
	gwPrefix := s.watchGwRegInfo.Prefix() + "/"
	return register.Watch(s.app.Context(), gwPrefix, func(key string, val string, isDel bool) {
		segments := strings.Split(key, "/")
		host := segments[len(segments)-1]
		if host != s.rpcServer.Host().String() {
			if isDel {
				s.logger.Debug(utils.ToStr("wss gateway [", host, "] leaved"))
				s.watchClient.Rm("gateway", host)
			} else {
				s.logger.Debug(utils.ToStr("wss gateway [", host, "] added"))
				s.watchClient.Add("gateway", host)
			}
		}
	})
}

func (s *Server) addErr(err error) {
	if err != nil {
		s.errs = append(s.errs, err)
	}
}

func (s *Server) RegisterGatewayServer(key string) error {
	if s.app.Register() != nil && key != "" {
		if err := s.app.Register().Register(s.app.Context(), key, s.host.String(), s.app.RegTtl()); err != nil {
			return utils.NewWrappedError(s.msg("register gateway failed"), err)
		}
	}

	return nil
}

func (s *Server) AuthEnabled() bool {
	return s.eventHandler.AuthEnabled()
}

func (s *Server) Security() bool {
	return s.eventHandler.Security()
}

func (s *Server) Host() url.Host {
	return s.host
}

func (s *Server) Manager() *manage.Manager {
	return s.manager
}

func (s *Server) ActionManager() *action.Manager {
	return s.actManager
}

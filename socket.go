package socketgateway

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/application/regtype"
	"github.com/obnahsgnaw/application/servertype"
	"github.com/obnahsgnaw/application/service/regCenter"
	rpc2 "github.com/obnahsgnaw/rpc"
	bindv1 "github.com/obnahsgnaw/socketapi/gen/bind/v1"
	connv1 "github.com/obnahsgnaw/socketapi/gen/conninfo/v1"
	groupv1 "github.com/obnahsgnaw/socketapi/gen/group/v1"
	messagev1 "github.com/obnahsgnaw/socketapi/gen/message/v1"
	slbv1 "github.com/obnahsgnaw/socketapi/gen/slb/v1"
	"github.com/obnahsgnaw/socketgateway/asset"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/net"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/action"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	gatewayv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/gateway/v1"
	"github.com/obnahsgnaw/socketgateway/service/proto/impl"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const closeAction = 0

// socket, rpc, doc, 注册socket-gateway、发现socket-gateway、发现socket-server

type Server struct {
	id              string // 模块-子模块
	name            string
	st              servertype.ServerType
	sct             sockettype.SocketType
	et              endtype.EndType
	host            url.Host
	dc              *DocConfig
	app             *application.Application
	m               *action.Manager
	ss              *socket.Server
	e               *eventhandler.Event
	eo              []eventhandler.Option
	se              socket.Engine
	regInfo         *regCenter.RegInfo
	rs              *rpc2.Server
	rsCus           bool
	ds              *DocServer
	dsCus           bool
	dsPrefixed      bool // 自定义外部引擎时，多个文档路径冲突，指定外部引擎时或者单独的引擎手动指定增加前缀保持一致（用于有个集中服务包含很多，又有单独的服务）
	logger          *zap.Logger
	handlerRegInfo  *regCenter.RegInfo
	errs            []error
	poll            bool
	regEnable       bool
	routeDebug      bool
	reuseAddr       bool
	keepalive       uint
	tickInterval    time.Duration
	authAddress     string
	crypto          eventhandler.Cryptor
	noAuthStaticKey []byte
	actListeners    []func(*action.Manager)
}

func st2hdt(sst servertype.ServerType) servertype.ServerType {
	switch sst {
	case servertype.Tcp:
		return servertype.TcpHdl
	case servertype.Wss:
		return servertype.WssHdl
	case servertype.Udp:
		return servertype.UdpHdl
	}
	panic("trans server type to handler type failed")
}

func New(app *application.Application, st sockettype.SocketType, et endtype.EndType, host url.Host, options ...Option) *Server {
	var err error
	s := &Server{
		id:   "gateway-gateway",
		name: "gateway",
		st:   st.ToServerType(),
		sct:  st,
		et:   et,
		host: host,
		app:  app,
		m:    action.NewManager(),
	}
	if s.st == "" {
		s.addErr(errors.New(s.msg("type not support")))
	}
	logCnf := logger.CopyCnfWithLevel(s.app.LogConfig())
	if logCnf != nil {
		logCnf.AddSubDir(filepath.Join(s.et.String(), utils.ToStr(s.st.String(), "-", s.id)))
		logCnf.SetFilename(utils.ToStr(s.st.String(), "-", s.id))
		logCnf.ReplaceTraceLevel(zap.NewAtomicLevelAt(zap.FatalLevel))
	}
	s.logger, err = logger.New(utils.ToStr(s.st.String(), "-gateway"), logCnf, app.Debugger().Debug())
	s.addErr(err)
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
			Type:    st2hdt(s.st).String(),
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

// new rpc server for socket
func (s *Server) withRpcServer(port int) *rpc2.Server {
	s.rs = rpc2.New(s.app, s.sct.String()+"-gateway", utils.ToStr(s.sct.String(), "-", s.id, "-rpc"), s.et, url.Host{
		Ip:   s.host.Ip,
		Port: port,
	}, rpc2.RegEnable(), rpc2.Parent(s))

	return s.rs
}

func (s *Server) initRpc() {
	if s.rs != nil {
		s.rs.RegisterService(rpc2.ServiceInfo{
			Desc: bindv1.BindService_ServiceDesc,
			Impl: impl.NewBindService(func() *socket.Server { return s.ss }),
		})
		s.rs.RegisterService(rpc2.ServiceInfo{
			Desc: connv1.ConnService_ServiceDesc,
			Impl: impl.NewConnService(func() *socket.Server { return s.ss }),
		})
		s.rs.RegisterService(rpc2.ServiceInfo{
			Desc: groupv1.GroupService_ServiceDesc,
			Impl: impl.NewGroupService(func() *socket.Server { return s.ss }, func() *eventhandler.Event { return s.e }),
		})
		s.rs.RegisterService(rpc2.ServiceInfo{
			Desc: messagev1.MessageService_ServiceDesc,
			Impl: impl.NewMessageService(func() *socket.Server { return s.ss }, func() *eventhandler.Event { return s.e }),
		})
		s.rs.RegisterService(rpc2.ServiceInfo{
			Desc: slbv1.SlbService_ServiceDesc,
			Impl: impl.NewSlbService(func() *socket.Server { return s.ss }),
		})
		s.m.With(action.CloseAction(closeAction))
		s.m.With(action.Gateway(s.rs.Host()))
		s.m.With(action.RtHandler(impl.NewRemoteHandler(s.logger)))
	}
}

func (s *Server) withRpcServerIns(ins *rpc2.Server) {
	s.rs = ins
	s.rsCus = true
	s.rs.AddRegInfo(s.sct.String()+"-gateway", utils.ToStr(s.sct.String(), "-", s.id, "-rpc"), s)
}

// withDocServerIns doc server
func (s *Server) withDocServerIns(e *gin.Engine, ePort int, docProxyPrefix string) {
	s.dsCus = true
	s.ds = NewDocServerWithEngine(e, s.app.ID(), s.docConfig(ePort, docProxyPrefix))
}

func (s *Server) withDocServer(port int, docProxyPrefix string, projPrefixed bool) *DocServer {
	s.dsPrefixed = projPrefixed
	s.ds = NewDocServer(s.app.ID(), s.docConfig(port, docProxyPrefix))
	return s.ds
}

func (s *Server) docConfig(port int, docProxyPrefix string) *DocConfig {
	if docProxyPrefix != "" {
		docProxyPrefix = "/" + strings.Trim(docProxyPrefix, "/")
	}

	docName := "docs"
	if s.dsCus || s.dsPrefixed {
		docName = utils.ToStr(s.et.String(), "-", s.st.String(), "-docs")
	}
	return &DocConfig{
		id:       s.id,
		endType:  s.et,
		servType: s.st,
		Origin: url.Origin{
			Protocol: url.HTTP,
			Host: url.Host{
				Ip:   s.host.Ip,
				Port: port,
			},
		},
		RegTtl:        s.app.RegTtl(),
		Prefix:        docProxyPrefix,
		socketGateway: true,
		Doc: DocItem{
			socketType: s.sct,
			Path:       utils.ToStr("/", docName, "/gateway/gateway."+s.st.String()+"doc"), // the same with the socket handler,
			Prefix:     docProxyPrefix + "/docs",
			Title:      s.name,
			Public:     true,
			Provider: func() ([]byte, error) {
				return asset.Asset("service/doc/html/gateway.html")
			},
		},
		debug: s.routeDebug,
	}
}

func (s *Server) watchLog(watcher eventhandler.LogWatcher) {
	s.regEo(eventhandler.Watcher(watcher))
}

func (s *Server) Engine() *socket.Server {
	return s.ss
}

func (s *Server) addTicker(name string, ticker eventhandler.TickHandler) {
	s.regEo(eventhandler.Ticker(name, ticker))
}

func (s *Server) Listen(act codec.Action, structure action.DataStructure, handler action.Handler) {
	s.actListeners = append(s.actListeners, func(manager *action.Manager) {
		manager.RegisterHandlerAction(act, structure, handler)
		s.logger.Debug("listened action:" + act.Name)
	})
}

func (s *Server) Send(c socket.Conn, id codec.Action, data codec.DataPtr) (err error) {
	return s.e.SendAction(c, id, data)
}

func (s *Server) regEo(option eventhandler.Option) {
	s.eo = append(s.eo, option)
}

func (s *Server) Rpc() *rpc2.Server {
	return s.rs
}

func (s *Server) Release() {
	var cb = func(msg string) {
		if s.logger != nil {
			s.logger.Debug(msg)
		}
	}
	if s.RegEnabled() && s.app.Register() != nil {
		_ = s.app.DoUnregister(s.regInfo, cb)
	}
	if s.ds != nil && s.app.Register() != nil {
		_ = s.app.DoUnregister(s.ds.regInfo, cb)
	}
	if s.rs != nil {
		s.rs.Release()
	}
	if s.logger != nil {
		s.logger.Info("released")
		_ = s.logger.Sync()
	}
}

func (s *Server) setSocketEngine(engine socket.Engine) {
	s.se = engine
}

func (s *Server) Run(failedCb func(error)) {
	if s.errs != nil {
		failedCb(s.errs[0])
		return
	}
	s.logger.Info("init start...")
	if s.se == nil {
		s.se = net.New()
		s.logger.Info("socket engine initialize(default)")
	} else {
		s.logger.Info("socket engine initialize(customer)")
	}
	s.regEo(eventhandler.Logger(s.logger))
	s.e = eventhandler.New(s.app.Context(), s.m, s.sct, s.eo...)
	s.ss = socket.New(s.app.Context(), s.sct, s.host.Port, s.se, s.e, &socket.Config{
		MultiCore: true,
		Keepalive: s.keepalive,
		NoDelay:   true,
		Ticker:    s.tickInterval > 0,
		ReuseAddr: s.reuseAddr,
	})
	s.logger.Info("socket server initialized")
	s.defaultListen()
	for _, h := range s.actListeners {
		h(s.m)
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
	if s.ds != nil {
		s.logger.Info("doc server enabled, init start...")
		s.logger.Info(utils.ToStr("doc index url=", s.ds.IndexDocUrl(), ", gateway doc url=", s.ds.DocUrl()))
		if !s.dsCus {
			docDesc := utils.ToStr("doc server[", s.ds.config.Origin.Host.String(), "] ")
			s.logger.Info(docDesc + "start and serving...")
			s.ds.SyncStart(failedCb)
		}
		if s.app.Register() != nil {
			s.logger.Debug("doc register start")
			if err := s.app.DoRegister(s.ds.regInfo, regLogCb); err != nil {
				failedCb(s.socketGwError(s.msg("register doc failed"), err))
			}
			s.logger.Debug("doc registered")
		}
		s.logger.Debug("sub doc watch start...")
		prefix := s.ds.regInfo.Prefix()
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
					s.ds.Manager.Remove(moduleName, keyName, val)
				}
			} else {
				if attr == "url" {
					s.logger.Debug(utils.ToStr("sub doc[", moduleName, ":", keyName, "] joined"))
					s.ds.Manager.Add(moduleName, keyName, "", val, nil)
				}
				if attr == "title" {
					s.ds.Manager.Add(moduleName, keyName, val, "", nil)
				}
				if attr == "public" {
					var public bool
					if val == "1" {
						public = true
					}
					s.ds.Manager.Add(moduleName, keyName, "", "", &public)
				}
			}
		})
		if err != nil {
			failedCb(err)
			return
		}
	}
	if s.rs != nil {
		s.logger.Info("rpc server enabled, init start...")
		s.initRpc()
		if !s.rsCus {
			s.rs.Run(failedCb)
		}
	}
	s.logger.Info(utils.ToStr("socket[", s.host.String(), "] start and serving..."))
	s.ss.SyncStart(failedCb)
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
				Success:  false,
				CryptKey: nil,
			}
			respData = response
			// 没有地址标识没启用认证 返回默认的密钥
			if s.authAddress == "" {
				response.Success = true
				// 有加解密返回默认密钥
				if s.crypto != nil {
					response.CryptKey = s.noAuthStaticKey
				}
			} else {
				// 有认证
				rq, err := http.NewRequest("GET", s.authAddress, nil)
				if err != nil {
					response.Success = false
					s.Logger().Error(s.msg("auth action forward error: err=" + err.Error()))
					return
				}
				rq.Header.Set("Authorization", "soc "+q.Token)
				client := &http.Client{}
				resp, err := client.Do(rq)
				if err != nil {
					s.Logger().Error(s.msg("auth action request resp error: err=" + err.Error()))
					response.Success = false
				}
				if resp.StatusCode >= http.StatusMultipleChoices {
					response.Success = false
				} else {
					response.Success = true
					uidStr := resp.Header.Get("X-User-Id")
					uid, _ := strconv.Atoi(uidStr)
					uname := resp.Header.Get("X-User-Name")
					c.Context().Auth(&socket.AuthUser{
						Id:   uint(uid),
						Name: uname,
					})
					key := s.crypto.Type().RandKey()
					c.Context().SetOptional("cryptKey", key)
					response.CryptKey = key
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
			s.logger.Debug(utils.ToStr("action [", moduleName, ":", keyName, "]", strconv.Itoa(actionId), " leaved"))
			s.m.UnregisterRemoteAction(host)
		} else {
			s.logger.Debug(utils.ToStr("action [", moduleName, ":", keyName, "]", strconv.Itoa(actionId), " added"))
			flbNum := ""
			if strings.Contains(val, "|") {
				valNum := strings.Split(val, "|")
				val = valNum[0]
				flbNum = valNum[1]
			}
			s.m.RegisterRemoteAction(codec.Action{
				Id:   codec.ActionId(actionId),
				Name: val,
			}, host, flbNum)
		}
	})
	if err != nil {
		return err
	}

	return nil
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

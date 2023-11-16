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
	bindv1 "github.com/obnahsgnaw/socketapi/gen/bind/v1"
	connv1 "github.com/obnahsgnaw/socketapi/gen/conninfo/v1"
	groupv1 "github.com/obnahsgnaw/socketapi/gen/group/v1"
	messagev1 "github.com/obnahsgnaw/socketapi/gen/message/v1"
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
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net/http"
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
	se              socket.Engine
	regInfo         *regCenter.RegInfo
	rs              *rpc2.Server
	ds              *DocServer
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
		panic("trans socket type to server type failed")
	}
	return
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
		st:   sct2st(st),
		sct:  st,
		et:   et,
		host: host,
		app:  app,
		m:    action.NewManager(),
	}
	if s.st == "" {
		s.addErr(errors.New(s.msg("type not support")))
	}
	s.logger, err = logger.New(utils.ToStr("Soc[", et.String(), "][", st.String(), "-gateway]"), app.LogConfig(), app.Debugger().Debug())
	s.addErr(err)
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

// WithRpcServer new rpc server for socket
func (s *Server) WithRpcServer(port int) *rpc2.Server {
	ss := rpc2.New(s.app, s.sct.String()+"-gateway", utils.ToStr(s.sct.String(), "-", s.id, "-rpc"), s.et, url.Host{
		Ip:   s.host.Ip,
		Port: port,
	}, rpc2.RegEnable(), rpc2.Parent(s))
	ss.RegisterService(rpc2.ServiceInfo{
		Desc: bindv1.BindService_ServiceDesc,
		Impl: impl.NewBindService(func() *socket.Server { return s.ss }),
	})
	ss.RegisterService(rpc2.ServiceInfo{
		Desc: connv1.ConnService_ServiceDesc,
		Impl: impl.NewConnService(func() *socket.Server { return s.ss }),
	})
	ss.RegisterService(rpc2.ServiceInfo{
		Desc: groupv1.GroupService_ServiceDesc,
		Impl: impl.NewGroupService(func() *socket.Server { return s.ss }, func() *eventhandler.Event { return s.e }),
	})
	ss.RegisterService(rpc2.ServiceInfo{
		Desc: messagev1.MessageService_ServiceDesc,
		Impl: impl.NewMessageService(func() *socket.Server { return s.ss }, func() *eventhandler.Event { return s.e }),
	})
	s.m.With(action.CloseAction(closeAction))
	s.m.With(action.Gateway(ss.Host()))
	s.m.With(action.RtHandler(impl.NewRemoteHandler()))
	s.rs = ss
	s.debug("rpc server enabled")

	return ss
}

// WithDocServer doc server
func (s *Server) WithDocServer(port int, docProxyPrefix string) *DocServer {
	if docProxyPrefix != "" {
		docProxyPrefix = "/" + strings.Trim(docProxyPrefix, "/")
	}
	config := &DocConfig{
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
			Path:       "/docs/gateway/gateway." + s.st.String() + "doc", // the same with the socket handler
			Prefix:     docProxyPrefix + "/docs",
			Title:      s.name,
			Public:     true,
			Provider: func() ([]byte, error) {
				return asset.Asset("service/doc/html/gateway.html")
			},
		},
		debug: s.routeDebug,
	}
	s.ds = NewDocServer(s.app.ID(), config)
	s.debug("doc server enabled")

	return s.ds
}

func (s *Server) WatchLog(watcher func(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field)) {
	s.e.WatchLog(watcher)
}

func (s *Server) Engine() *socket.Server {
	return s.ss
}

func (s *Server) AddTicker(name string, ticker eventhandler.TickHandler) {
	s.e.AddTicker(name, ticker)
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
	if s.RegEnabled() && s.app.Register() != nil {
		_ = s.app.DoUnregister(s.regInfo)
	}
	if s.ds != nil && s.app.Register() != nil {
		_ = s.app.DoUnregister(s.ds.regInfo)
	}
	if s.rs != nil {
		s.rs.Release()
	}
	_ = s.logger.Sync()
	s.debug("released")
}

func (s *Server) SetSocketEngine(engine socket.Engine) {
	s.se = engine
}

func (s *Server) Run(failedCb func(error)) {
	if s.errs != nil {
		failedCb(s.errs[0])
		return
	}
	s.debug("start running...")
	if s.se == nil {
		s.se = net.New()
	}
	s.ss = socket.New(s.app.Context(), s.sct, s.host.Port, s.se, s.e, &socket.Config{
		MultiCore: true,
		Keepalive: s.keepalive,
		NoDelay:   true,
		Ticker:    s.tickInterval > 0,
		ReuseAddr: s.reuseAddr,
	})
	if s.app.Register() != nil {
		if s.RegEnabled() {
			if err := s.app.DoRegister(s.regInfo); err != nil {
				failedCb(s.socketGwError(s.msg("register soc failed"), err))
			}
		}
		if err := s.app.DoRegister(s.ds.regInfo); err != nil {
			failedCb(s.socketGwError(s.msg("register doc failed"), err))
		}
		if err := s.watch(s.app.Register()); err != nil {
			failedCb(s.socketGwError(s.msg("watch failed"), err))
		}
	}
	s.defaultListen()
	s.logger.Info(utils.ToStr(s.st.String(), " server[", s.host.String(), "] start and serving..."))
	s.ss.SyncStart(failedCb)
	if s.ds != nil {
		docDesc := utils.ToStr("doc server[", s.ds.config.Origin.Host.String(), "] ")
		s.logger.Info(docDesc + "start and serving...")
		s.debug("doc url=" + s.ds.DocUrl())
		s.debug("index doc url=" + s.ds.IndexDocUrl())
		s.ds.SyncStart(failedCb)
	}
	if s.rs != nil {
		s.rs.Run(failedCb)
	}
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

func (s *Server) debug(msg string) {
	if s.app.Debugger().Debug() {
		s.logger.Debug(msg)
	}
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

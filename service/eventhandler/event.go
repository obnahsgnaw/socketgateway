package eventhandler

import (
	"bytes"
	"context"
	"crypto"
	"encoding/binary"
	"errors"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/goutils/security/coder"
	"github.com/obnahsgnaw/goutils/security/esutil"
	"github.com/obnahsgnaw/goutils/security/rsautil"
	"github.com/obnahsgnaw/socketgateway/pkg/group"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/action"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler/connutil"
	gatewayv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/gateway/v1"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strconv"
	"time"
)

// Event 事件处理引擎
type Event struct {
	ctx           context.Context
	am            *action.Manager
	tickHandlers  map[string]TickHandler
	logWatcher    func(socket.Conn, string, zapcore.Level, ...zap.Field)
	logger        *zap.Logger
	st            sockettype.SocketType
	codecProvider codec.Provider
	codedProvider codec.DataBuilderProvider
	authEnable    bool
	tickInterval  time.Duration
	privateKey    []byte
	defaultUser   *socket.AuthUser
	interceptor   func() error
	rsa           *rsautil.Rsa // 连接后第一包发送rsa加密 aes密钥@时间戳 交换密钥， 服务端 rsa解密得到aes 密钥， 后续使用其来解析aes cbc 加密的内容体， 内容体组成为：iv（16byte）+内容 (aes256 最小16字节) 一个内容最小32字节
	es            *esutil.ADes
	esTp          esutil.EsType
	esMode        esutil.EsMode
	secEncoder    coder.Encoder
	secEncode     bool
	secTtl        int64 // second
	ss            *socket.Server
	authProviders map[string]AuthenticateProvider
}

// AuthenticateProvider 返回user即相当于做了用户认证
type AuthenticateProvider func(auth *socket.Authentication, encryptedKey []byte, encoder coder.Encoder, encoded, secEnable bool) (user *socket.AuthUser, decryptedKey []byte, err error)

type LogWatcher func(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field)

func New(ctx context.Context, m *action.Manager, st sockettype.SocketType, options ...Option) *Event {
	s := &Event{
		ctx:           ctx,
		am:            m,
		st:            st,
		tickHandlers:  make(map[string]TickHandler),
		rsa:           rsautil.New(rsautil.PKCS1Public(), rsautil.PKCS1Private(), rsautil.SignHash(crypto.SHA256), rsautil.Encoder(coder.B64StdEncoding)),
		es:            esutil.New(esutil.Aes256, esutil.CbcMode, esutil.Encoder(coder.B64StdEncoding)),
		esTp:          esutil.Aes256,
		esMode:        esutil.CbcMode,
		secEncoder:    coder.B64StdEncoding,
		secTtl:        60,
		authProviders: make(map[string]AuthenticateProvider),
	}
	toData := func(p *codec.PKG) codec.DataPtr {
		if p == nil {
			return &gatewayv1.GatewayPackage{}
		}
		return &gatewayv1.GatewayPackage{
			Action: p.Action.Val(),
			Data:   p.Data,
		}
	}
	toPkg := func(d codec.DataPtr) *codec.PKG {
		d1 := d.(*gatewayv1.GatewayPackage)
		return &codec.PKG{
			Action: codec.ActionId(d1.Action),
			Data:   d1.Data,
		}
	}
	if s.st.IsTcp() {
		s.codecProvider = codec.TcpDefaultProvider(toData, toPkg)
	} else if s.st.IsUdp() {
		s.codecProvider = codec.UdpDefaultProvider(toData, toPkg)
	} else {
		s.codecProvider = codec.WssDefaultProvider(toData, toPkg)
	}
	s.codedProvider = codec.NewDbp()
	s.With(options...)
	s.AddTicker("authenticate-ticker", authenticateTicker(time.Second*10))
	s.RegisterAuthenticate("user", func(auth *socket.Authentication, encryptedKey []byte, encoder coder.Encoder, decoded, secEnable bool) (user *socket.AuthUser, decryptedKey []byte, err error) {
		if secEnable {
			decryptedKey, err = s.rsa.Decrypt(encryptedKey, s.privateKey, decoded)
		}
		if !s.authEnable {
			user = s.defaultUser
		}
		return
	})
	return s
}

func (e *Event) With(options ...Option) {
	for _, o := range options {
		o(e)
	}
}

func (e *Event) OnBoot(s *socket.Server) {
	e.ss = s
	if e.logger != nil {
		e.logger.Info(utils.ToStr(s.Type().String(), " service[", strconv.Itoa(s.Port()), "] booted"))
	}
	e.proxyTick(e.ctx)
}

func (e *Event) OnOpen(s *socket.Server, c socket.Conn) {
	defer utils.RecoverHandler("on open", func(err, stack string) {
		if e.logger != nil {
			e.logger.Error("on open err=" + err + ", stack=" + stack)
		}
	})
	rqId := utils.GenLocalId("rq")
	e.log(c, rqId, "Connected", zapcore.InfoLevel)
	if e.interceptor != nil {
		if err := e.interceptor(); err != nil {
			e.log(c, "", "intercepted:"+err.Error(), zapcore.WarnLevel)
			connutil.SetCloseReason(c, "close by interceptor: "+err.Error())
			c.Close()
		}
	}
}

func (e *Event) OnClose(s *socket.Server, c socket.Conn, err error) {
	defer utils.RecoverHandler("on close", func(err, stack string) {
		if e.logger != nil {
			e.logger.Error("on close err=" + err + ", stack=" + stack)
		}
	})
	reason := ""
	if err != nil {
		reason = err.Error()
	} else {
		reason = connutil.GetCloseReason(c)
		if reason == "" {
			reason = "unknown"
		}
	}
	e.log(c, "", "Disconnected, reason="+reason, zapcore.InfoLevel)
	e.am.HandleClose(c)
	// 退组
	s.Groups().RangeGroups(func(g *group.Group) bool {
		g.Leave(c.Fd())
		return true
	})
}

func (e *Event) OnTraffic(_ *socket.Server, c socket.Conn) {
	defer utils.RecoverHandler("on traffic", func(err, stack string) {
		if e.logger != nil {
			e.logger.Error(" on traffic err=" + err + ", stack=" + stack)
		}
	})

	rqId := utils.GenLocalId("rq")
	// Read the data
	rawPkg, err := c.Read()
	if err != nil {
		e.log(c, rqId, "read failed, err="+err.Error(), zapcore.WarnLevel)
		return
	}

	// Initialize the encryption and decryption key
	keyHit, secResponse, initPackage, secErr := e.authenticate(c, rqId, rawPkg)
	if keyHit {
		_ = c.Write([]byte(secResponse))
		if secErr != nil {
			e.log(c, rqId, "crypt key parse failed, err="+secErr.Error(), zapcore.WarnLevel)
		} else {
			e.log(c, rqId, "crypt key parse success", zapcore.DebugLevel)
		}
		return
	}
	if len(initPackage) == 0 {
		return
	}

	// Unpacking a protocol package
	err = e.codecDecode(c, initPackage, func(packedPkg []byte) {
		e.log(c, rqId, "package received", zapcore.DebugLevel, zap.ByteString("package", packedPkg))
		rqAction, respAction, rqData, respData, respPackage, err1 := e.handleMessage(c, rqId, packedPkg)
		if err1 != nil {
			e.log(c, rqId, "package handled: request action="+rqAction.String()+err1.Error(), zapcore.WarnLevel)
		} else {
			e.log(c, rqId, "package handled: request action="+rqAction.String()+", response action="+respAction.String(), zapcore.InfoLevel, zap.String("rq_action", rqAction.String()), zap.String("resp_action", respAction.String()), zap.ByteString("rq_data", rqData), zap.ByteString("resp_data", respData))
		}
		if len(respPackage) > 0 {
			if err1 = e.write(c, respPackage); err1 != nil {
				e.log(c, rqId, err1.Error(), zapcore.ErrorLevel)
			}
		} else {
			e.log(c, rqId, "handle success, but no response", zapcore.InfoLevel)
		}
	})
	if err != nil {
		e.log(c, rqId, "codec decode failed, err="+err.Error(), zapcore.WarnLevel, zap.ByteString("package", initPackage))
		if err = e.gatewayErrorResponse(c, rqId, gatewayv1.GatewayError_PackageErr, 0); err != nil {
			e.log(c, rqId, err.Error(), zapcore.ErrorLevel)
		}
	}
}

func (e *Event) OnTick(s *socket.Server) (delay time.Duration) {
	defer utils.RecoverHandler("on tick", func(err, stack string) {
		if e.logger != nil {
			e.logger.Error("on tick err=" + err + ", stack=" + stack)
		}
	})
	s.RangeConnections(func(c socket.Conn) bool {
		for _, h := range e.tickHandlers {
			if !h(s, c) {
				break
			}
		}
		return true
	})

	return e.tickInterval
}

func (e *Event) OnShutdown(s *socket.Server) {
	if e.logger != nil {
		e.logger.Info(utils.ToStr(s.Type().String(), "service[", strconv.Itoa(s.Port()), "] down"))
	}
}

// HandleMessage handle message
func (e *Event) handleMessage(c socket.Conn, rqId string, packedPkg []byte) (rqAction, respAction codec.Action, rqData, respData, respPackage []byte, err error) {
	var err1 error
	// Decrypt the packet
	decryptedData, decErr := e.decrypt(c, packedPkg)
	if decErr != nil {
		err = errors.New("decrypt data failed, err=" + decErr.Error())
		if respPackage, err1 = e.packGatewayError(c, gatewayv1.GatewayError_DecryptErr, 0); err1 != nil {
			err = errors.New(err.Error() + ":" + err1.Error())
		}
		return
	}

	// Decode the action
	gwPkg, acErr := e.actionDecode(c, decryptedData)
	if acErr != nil {
		err = errors.New("action decode failed, err=" + acErr.Error())
		if respPackage, err1 = e.packGatewayError(c, gatewayv1.GatewayError_ActionErr, 0); err1 != nil {
			err = errors.New(err.Error() + ":" + err1.Error())
		}
		return
	}
	rqData = gwPkg.Data

	// Check the registration status of the action handler
	var ok bool
	if rqAction, ok = e.am.GetAction(gwPkg.Action); !ok {
		err = errors.New("handler not found, raw request action=" + gwPkg.Action.String())
		if respPackage, err1 = e.packGatewayError(c, gatewayv1.GatewayError_NoActionHandler, gwPkg.Action.Val()); err1 != nil {
			err = errors.New(err.Error() + ":" + err1.Error())
		}
		return
	}

	// Authentication checks, excluding certification-related actions
	if !e.authCheck(c, gatewayv1.ActionId(gwPkg.Action)) {
		err = errors.New("handle failed, no auth")
		if respPackage, err1 = e.packGatewayError(c, gatewayv1.GatewayError_NoAuth, gwPkg.Action.Val()); err1 != nil {
			err = errors.New(err.Error() + ":" + err1.Error())
		}
		return
	}

	// Distribute the action to the handler for processing and obtain the response information
	var dispatchErr error
	dataCoder := e.DataCoder(c)
	if respAction, respData, dispatchErr = e.am.Dispatch(c, rqId, dataCoder, gwPkg.Action, gwPkg.Data); dispatchErr != nil {
		// Handle the situation that the processing cannot be found according to the status code (usually this does not happen)
		if st, ok1 := status.FromError(dispatchErr); ok1 {
			if st.Code() == codes.NotFound {
				err = errors.New("package dispatch failed,err=no action handler")
				if respPackage, err1 = e.packGatewayError(c, gatewayv1.GatewayError_NoActionHandler, gwPkg.Action.Val()); err1 != nil {
					err = errors.New(err.Error() + ":" + err1.Error())
				}
				return
			}
		}

		err = errors.New("package dispatch failed,err=" + dispatchErr.Error())
		if respPackage, err1 = e.packGatewayError(c, gatewayv1.GatewayError_InternalErr, gwPkg.Action.Val()); err1 != nil {
			err = errors.New(err.Error() + ":" + err1.Error())
		}
		return
	}

	// Encode response data
	if respAction.Id > 0 {
		respPackage, err = e.pack(c, respAction, respData)
	}

	return
}

func (e *Event) log(c socket.Conn, rqId, msg string, l zapcore.Level, data ...zap.Field) {
	if rqId != "" {
		data = append(data, zap.String("rq_id", rqId))
	}
	if e.logWatcher != nil {
		e.logWatcher(c, msg, l, data...)
	} else {
		e.logger.Log(l, msg, data...)
	}
}

func (e *Event) WatchLog(watcher LogWatcher) {
	e.logWatcher = watcher
}

func (e *Event) AddTicker(name string, ticker TickHandler) {
	e.tickHandlers[name] = ticker
}

func (e *Event) initCodec(c socket.Conn, rqId string, name codec.Name) {
	dataCoderName, protoCoder, gatewayPkgCoder := e.codecProvider.GetByName(name)
	e.SetCoder(c, protoCoder, gatewayPkgCoder, e.codedProvider.Provider(dataCoderName))
	e.log(c, rqId, utils.ToStr("data format=", dataCoderName.String()), zapcore.InfoLevel)
}

func (e *Event) RegisterAuthenticate(name string, provider AuthenticateProvider) {
	e.authProviders[name] = provider
}

// 类型@标识@类型:{key 时间戳}
func (e *Event) authenticate(c socket.Conn, rqId string, pkg []byte) (hit bool, response string, initPackage []byte, err error) {
	if !e.CryptoKeyInitialized(c) {
		// 处理proxy
		if pkg = e.initProxy(pkg); len(pkg) == 0 {
			return
		}

		hit = true
		var authentication *socket.Authentication
		codeType := codec.Proto
		if bytes.Contains(pkg, []byte("::")) {
			parts := bytes.SplitN(pkg, []byte("::"), 2)
			typeIdentify := parts[0]
			pkg = parts[1]
			if !bytes.Contains(typeIdentify, []byte("@")) {
				response = "222"
				err = errors.New("type identify key format error")
				return
			}
			if parts = bytes.SplitN(typeIdentify, []byte("@"), 3); len(parts) != 3 {
				response = "222"
				err = errors.New("type identify format error")
				return
			}
			authentication = &socket.Authentication{Type: string(parts[0]), Id: string(parts[1])}
			if string(parts[2]) == codec.Json.String() {
				codeType = codec.Json
			}
		} else {
			authentication = &socket.Authentication{Type: "user", Id: ""}
			codeType = codec.Json
		}

		var keys []byte
		var user *socket.AuthUser
		if ap, ok := e.authProviders[authentication.Type]; !ok {
			response = "222"
			err = errors.New("authentication types that are not supported")
			return
		} else {
			if user, keys, err = ap(authentication, pkg, e.secEncoder, e.secEncode, e.Security()); err != nil {
				response = "222"
				return
			}
		}

		var key []byte
		if e.Security() {
			if len(keys) != e.es.Type().KeyLen()+10 {
				response = "222"
				err = errors.New("invalid crypt key length")
				return
			}
			key = keys[:e.es.Type().KeyLen()]
			timestamp := keys[e.es.Type().KeyLen():]
			timestampInt, err1 := strconv.Atoi(string(timestamp))
			if err1 != nil {
				response = "222"
				err = errors.New("invalid crypt key timestamp")
				return
			}
			timeLimit := time.Now().Unix() - int64(timestampInt)
			if timeLimit > e.secTtl || timeLimit < -e.secTtl {
				response = "222"
				err = errors.New("invalid crypt timestamp expire")
				return
			}
			response = "111"
		} else {
			key = []byte("")
			response = "000"
		}

		e.log(c, rqId, utils.ToStr("authenticate type=", authentication.Type, "authenticate id=", authentication.Id), zapcore.InfoLevel)
		c.Context().Authenticate(authentication)
		if user != nil {
			c.Context().Auth(user)
			e.ss.BindId(c, socket.ConnId{
				Id:   strconv.Itoa(int(user.Id)),
				Type: "UID",
			})
			e.log(c, rqId, utils.ToStr("authenticate with user=", user.Name), zapcore.InfoLevel)
		}
		e.SetCryptoKey(c, key)
		if len(key) > 0 {
			e.log(c, rqId, "authenticate with security", zapcore.InfoLevel)
		}
		e.initCodec(c, rqId, codeType)
		return
	}
	initPackage = pkg
	return
}

// proxy protocol v1: PROXY TCP4 202.112.144.236 10.210.12.10 5678 80\r\n
// proxy protocol v2: [13 10 13 10 0 13 10 81 85 73 84 10 33 17 0 12 127 0 0 1 127 0 0 1 245 207 115 68]
// 12字节固定头+2字节+2字节长度值+长度内容， 即 16字节+15，16字节存储的长度
func (e *Event) initProxy(pkg []byte) []byte {
	if bytes.HasPrefix(pkg, []byte("PROXY")) {
		//v1
		if bytes.HasSuffix(pkg, []byte("\r\n")) {
			return nil
		} else {
			p1 := bytes.Split(pkg, []byte("\r\n"))
			return p1[1]
		}
	}
	v2 := []byte{0x0d, 0x0a, 0x0d, 0x0a, 0x00, 0x0d, 0x0a, 0x51, 0x55, 0x49, 0x54, 0x0a}

	if bytes.HasPrefix(pkg, v2) {
		l := pkg[14:16]
		ll := int(16 + binary.BigEndian.Uint16(l))
		if ll == len(pkg) {
			return nil
		} else {
			return pkg[ll:]
		}
	}

	return pkg
}

func (e *Event) ProtoCoder(c socket.Conn) codec.Codec {
	return connutil.ProtoCoder(c)
}

func (e *Event) GatewayPkgCoder(c socket.Conn) codec.PkgBuilder {
	return connutil.GatewayPkgCoder(c)
}

func (e *Event) DataCoder(c socket.Conn) codec.DataBuilder {
	return connutil.DataCoder(c)
}

func (e *Event) SetCoder(c socket.Conn, protoCoder codec.Codec, gatewayPkgCoder codec.PkgBuilder, dataCoder codec.DataBuilder) {
	connutil.SetCoder(c, protoCoder, gatewayPkgCoder, dataCoder)
}

func (e *Event) ClearCoder(c socket.Conn) {
	connutil.ClearCoder(c)
}

func (e *Event) CoderInitialized(c socket.Conn) bool {
	return connutil.CoderInitialized(c)
}

func (e *Event) SetCryptoKey(c socket.Conn, key []byte) {
	connutil.SetCryptoKey(c, key)
}

func (e *Event) ClearCryptoKey(c socket.Conn) {
	connutil.ClearCryptoKey(c)
}

func (e *Event) CryptoKey(c socket.Conn) []byte {
	return connutil.CryptoKey(c)
}

func (e *Event) CryptoKeyInitialized(c socket.Conn) bool {
	return connutil.CryptoKeyInitialized(c)
}

func (e *Event) codecDecode(c socket.Conn, pkg []byte, handler func(pkg []byte)) error {
	return connutil.WithTempPackager(c, pkg, func(kg1 []byte) ([]byte, error) {
		return e.ProtoCoder(c).Unmarshal(pkg, handler)
	})
}

func (e *Event) codecEncode(c socket.Conn, pkg []byte) ([]byte, error) {
	return e.ProtoCoder(c).Marshal(pkg)
}

func (e *Event) actionDecode(c socket.Conn, pkg []byte) (*codec.PKG, error) {
	p, err := e.GatewayPkgCoder(c).Unpack(pkg)
	if err != nil {
		return nil, err
	}
	if p.Action.Val() <= 0 {
		return nil, errors.New("action id zero")
	}

	return p, nil
}

func (e *Event) actionEncode(c socket.Conn, gwPkg *codec.PKG) ([]byte, error) {
	return e.GatewayPkgCoder(c).Pack(gwPkg)
}

func (e *Event) authCheck(c socket.Conn, actionId gatewayv1.ActionId) bool {
	if !e.authEnable {
		return true
	}

	if actionId == gatewayv1.ActionId_AuthReq || actionId == gatewayv1.ActionId_Ping {
		return true
	}

	return c.Context().Authed()
}

func (e *Event) decrypt(c socket.Conn, pkg []byte) (b []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err, _ = r.(error)
		}
	}()
	key := e.CryptoKey(c)
	if !e.Security() || len(pkg) == 0 || len(key) == 0 {
		return pkg, nil
	}
	index := e.es.Type().IvLen()
	if len(pkg) < index {
		return nil, errors.New("invalid data")
	}
	iv := pkg[0:index]
	pkg = pkg[index:]
	if len(pkg) == 0 {
		return pkg, nil
	}
	return e.es.Decrypt(pkg, key, iv, e.secEncode)
}

func (e *Event) encrypt(c socket.Conn, pkg []byte) ([]byte, error) {
	key := e.CryptoKey(c)
	if !e.Security() || len(pkg) == 0 || len(key) == 0 {
		return pkg, nil
	}
	data, iv, err := e.es.Encrypt(pkg, key, e.secEncode)
	if err != nil {
		return nil, err
	}
	data = append(iv, data...)

	return data, nil
}

func (e *Event) dataEncode(c socket.Conn, data codec.DataPtr) (b []byte, err error) {
	return e.DataCoder(c).Pack(data)
}

func (e *Event) pack(c socket.Conn, action codec.Action, data []byte) (packData []byte, err error) {
	// gateway pack
	if data, err = e.actionEncode(c, &codec.PKG{Action: action.Id, Data: data}); err != nil {
		err = utils.NewWrappedError("gateway package encode failed", err)
		return
	}

	// encrypt
	if data, err = e.encrypt(c, data); err != nil {
		err = utils.NewWrappedError("data encrypt failed", err)
		return
	}

	// codec
	if data, err = e.codecEncode(c, data); err != nil {
		err = utils.NewWrappedError("codec encode failed", err)
		return
	}
	packData = data
	return
}

func (e *Event) packRaw(c socket.Conn, action codec.Action, data codec.DataPtr) (packData []byte, err error) {
	if packData, err = e.dataEncode(c, data); err != nil {
		err = utils.NewWrappedError("data encode failed", err)
	}
	return e.pack(c, action, packData)
}

func (e *Event) packGatewayError(c socket.Conn, stat gatewayv1.GatewayError_Status, triggerActionId uint32) ([]byte, error) {
	return e.packRaw(c, action.New(gatewayv1.ActionId_GatewayErr), &gatewayv1.GatewayError{Status: stat, TriggerAction: triggerActionId})
}

func (e *Event) gatewayErrorResponse(c socket.Conn, rqId string, errorStatus gatewayv1.GatewayError_Status, triggerActionId uint32) error {
	if respPackage, err1 := e.packGatewayError(c, errorStatus, triggerActionId); err1 != nil {
		e.log(c, rqId, "package gateway error failed, err="+err1.Error(), zapcore.ErrorLevel)
	} else {
		if err1 = e.write(c, respPackage); err1 != nil {
			e.log(c, rqId, err1.Error(), zapcore.ErrorLevel)
		}
	}
	return nil
}

func (e *Event) write(c socket.Conn, data []byte) (err error) {
	if err = c.Write(data); err != nil {
		err = utils.NewWrappedError("write failed", err)
	}

	return
}

// Send Sends data that the packet has already encoded，It is mainly used to send encoded data sent by a handler
func (e *Event) Send(c socket.Conn, rqId string, a codec.Action, data []byte) (err error) {
	if data, err = e.pack(c, a, data); err != nil {
		e.log(c, rqId, "pack action failed, err="+err.Error(), zapcore.ErrorLevel, zap.String("action", a.String()), zap.ByteString("pkg", data))
		return
	}
	if err = e.write(c, data); err != nil {
		e.log(c, rqId, "send action failed, err="+err.Error(), zapcore.ErrorLevel, zap.String("action", a.String()), zap.ByteString("pkg", data))
	} else {
		e.log(c, "", "sent action["+a.String()+"]", zapcore.InfoLevel)
	}
	return
}

// SendAction Sends data that the packet has not yet encoded，It is mainly used for sending raw data to the current gateway
func (e *Event) SendAction(c socket.Conn, a codec.Action, data codec.DataPtr) (err error) {
	var packData []byte
	if packData, err = e.packRaw(c, a, data); err != nil {
		e.log(c, "", "pack action failed, err="+err.Error(), zapcore.ErrorLevel, zap.String("action", a.String()), zap.Any("pkg", data))
		return
	}

	if err = e.write(c, packData); err != nil {
		e.log(c, "", "send action failed, err="+err.Error(), zapcore.ErrorLevel, zap.String("action", a.String()), zap.ByteString("pkg", packData))
	} else {
		e.log(c, "", "sent action["+a.String()+"]", zapcore.InfoLevel)
	}
	return
}

func (e *Event) AuthEnabled() bool {
	return e.authEnable
}

func (e *Event) Security() bool {
	return len(e.privateKey) > 0
}

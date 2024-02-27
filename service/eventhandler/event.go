package eventhandler

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/socketgateway/pkg/group"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/action"
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
	crypto        Cryptor
	staticEsKey   []byte // 认证之前采用固定密钥，认证之后采用认证后的密钥
	defaultUser   *socket.AuthUser
}

type LogWatcher func(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field)

func New(ctx context.Context, m *action.Manager, st sockettype.SocketType, options ...Option) *Event {
	s := &Event{
		ctx:          ctx,
		am:           m,
		st:           st,
		tickHandlers: make(map[string]TickHandler),
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
	return s
}

func (e *Event) With(options ...Option) {
	for _, o := range options {
		o(e)
	}
}

func (e *Event) OnBoot(s *socket.Server) {
	if e.logger != nil {
		e.logger.Info(utils.ToStr(s.Type().String(), " service[", strconv.Itoa(s.Port()), "] booted"))
	}
}

func (e *Event) OnOpen(s *socket.Server, c socket.Conn) {
	e.log(c, "Connected", zapcore.InfoLevel)
	if e.crypto != nil {
		c.Context().SetOptional("cryptKey", e.staticEsKey)
	}
	if !e.authEnable {
		c.Context().Auth(e.defaultUser)
		s.BindId(c, socket.ConnId{
			Id:   strconv.Itoa(c.Fd()),
			Type: "UID",
		})
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
		r, ok := c.Context().GetOptional("close_reason")
		if ok {
			reason = r.(string)
		} else {
			reason = "unknown"
		}
	}
	e.log(c, "Disconnected, reason="+reason, zapcore.InfoLevel)
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
			e.logger.Error("on traffic err=" + err + ", stack=" + stack)
		}
	})
	if e.st.IsUdp() && e.crypto != nil {
		c.Context().SetOptional("cryptKey", e.staticEsKey)
	}
	// 读取数据
	rawPkg, err := c.Read()
	if err != nil {
		e.log(c, "read failed, err="+err.Error(), zapcore.ErrorLevel)
		return
	}
	// 获取设置 拆包器、action解码器、数据解码器
	coderName, unpacker, pkgBuilder, initdPkg := e.initCodec(c, rawPkg)
	if len(initdPkg) == 0 {
		return
	}
	// 拆包
	err = e.codecDecode(c, unpacker, initdPkg, func(packedPkg []byte) {
		e.log(c, "package received", zapcore.DebugLevel, zap.ByteString("package", packedPkg))

		// 解密
		decryptedData, err := e.decrypt(c, packedPkg)
		if err != nil {
			e.log(c, "decrypt data failed, err="+err.Error(), zapcore.ErrorLevel)
			if err = e.gatewayErrorResponse(c, gatewayv1.GatewayError_DecryptErr, 0); err != nil {
				e.log(c, err.Error(), zapcore.ErrorLevel)
			}
			return
		}

		// 解码action
		gwPkg, err := e.actionDecode(pkgBuilder, decryptedData)
		if err != nil {
			e.log(c, "action decode failed, err="+err.Error(), zapcore.ErrorLevel)
			if err = e.gatewayErrorResponse(c, gatewayv1.GatewayError_ActionErr, 0); err != nil {
				e.log(c, err.Error(), zapcore.ErrorLevel)
			}
			return
		}
		// 获取action
		rqAction, ok := e.am.GetAction(gwPkg.Action)
		if !ok {
			e.log(c, "handler not found, action="+(gwPkg.Action.String()), zapcore.ErrorLevel, zap.Uint32("action", gwPkg.Action.Val()))
			if err = e.gatewayErrorResponse(c, gatewayv1.GatewayError_NoActionHandler, gwPkg.Action.Val()); err != nil {
				e.log(c, err.Error(), zapcore.ErrorLevel)
			}
			return
		}
		e.log(c, "request action="+rqAction.String(), zapcore.InfoLevel, zap.Uint32("action_id", uint32(rqAction.Id)), zap.String("action_name", rqAction.Name))
		// 认证检查， 排除认证相关action
		if !e.authCheck(c, gatewayv1.ActionId(gwPkg.Action)) {
			e.log(c, "handle failed, no auth", zapcore.ErrorLevel)
			if err = e.gatewayErrorResponse(c, gatewayv1.GatewayError_NoAuth, gwPkg.Action.Val()); err != nil {
				e.log(c, err.Error(), zapcore.ErrorLevel)
			}
			return
		}

		// 分发action 获得响应信息
		respAction, respData, err := e.am.Dispatch(c, coderName, e.DataBuilder(c), gwPkg.Action, gwPkg.Data)
		if err != nil {
			if st, ok := status.FromError(err); ok {
				if st.Code() == codes.NotFound {
					e.log(c, "package dispatch failed,err="+err.Error(), zapcore.ErrorLevel)
					if err = e.gatewayErrorResponse(c, gatewayv1.GatewayError_NoActionHandler, uint32(rqAction.Id)); err != nil {
						e.log(c, err.Error(), zapcore.ErrorLevel)
					}
					return
				}
			}
			e.log(c, "package dispatch failed,err="+err.Error(), zapcore.ErrorLevel)
			if err = e.gatewayErrorResponse(c, gatewayv1.GatewayError_InternalErr, uint32(rqAction.Id)); err != nil {
				e.log(c, err.Error(), zapcore.ErrorLevel)
			}
			return
		}
		// 回复
		if respAction.Id > 0 {
			e.log(c, "response action="+respAction.String(), zapcore.InfoLevel, zap.Uint32("action_id", uint32(respAction.Id)), zap.String("action_name", respAction.Name), zap.ByteString("data", respData))
			if err = e.gatewayWrite(c, respAction, respData); err != nil {
				e.log(c, "response failed,err="+err.Error(), zapcore.ErrorLevel)
			} else {
				e.log(c, "response success", zapcore.InfoLevel)
			}
			return
		}
		e.log(c, "handle success, but no response", zapcore.InfoLevel)
	})
	if err != nil {
		e.log(c, "codec decode failed, err="+err.Error(), zapcore.ErrorLevel, zap.ByteString("package", initdPkg))
		if err = e.gatewayErrorResponse(c, gatewayv1.GatewayError_PackageErr, 0); err != nil {
			e.log(c, err.Error(), zapcore.ErrorLevel)
		}
		return
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

func (e *Event) log(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field) {
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

func (e *Event) initCodec(c socket.Conn, pkg []byte) (coderName codec.Name, unpacker codec.Codec, pkgBuilder codec.PkgBuilder, initdPkg []byte) {
	if p, ok := c.Context().GetOptional("codec"); ok {
		unpacker = p.(codec.Codec)
		pb, _ := c.Context().GetOptional("pkgBuilder")
		n, _ := c.Context().GetOptional("coderName")
		pkgBuilder = pb.(codec.PkgBuilder)
		coderName = n.(codec.Name)
		initdPkg = pkg
	} else {
		// 处理proxy
		pkg = e.initProxy(pkg)
		if len(pkg) == 0 {
			initdPkg = nil
			return
		}
		coderName, unpacker, pkgBuilder, initdPkg = e.codecProvider(pkg)
		c.Context().SetOptional("codec", unpacker)
		c.Context().SetOptional("pkgBuilder", pkgBuilder)
		c.Context().SetOptional("coderName", coderName)
		c.Context().SetOptional("dataBuilder", e.codedProvider.Provider(coderName))
		e.log(c, utils.ToStr("data format=", coderName.String()), zapcore.InfoLevel)
	}

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

func (e *Event) DataBuilder(c socket.Conn) codec.DataBuilder {
	v, _ := c.Context().GetOptional("dataBuilder")
	return v.(codec.DataBuilder)
}

func (e *Event) codecDecode(c socket.Conn, unpacker codec.Codec, pkg []byte, handler func(pkg []byte)) error {
	// 拼包
	if v, ok := c.Context().GetAndDelOptional("pkgTmp"); ok {
		pkg = append(v.([]byte), pkg...)
	}
	temp, err := unpacker.Unmarshal(pkg, handler)
	if err != nil {
		return err
	} else {
		c.Context().SetOptional("pkgTmp", temp)
	}
	return nil
}

func (e *Event) codecEncode(c socket.Conn, pkg []byte) ([]byte, error) {
	p, _ := c.Context().GetOptional("codec")
	unpacker := p.(codec.Codec)
	return unpacker.Marshal(pkg)
}

func (e *Event) actionDecode(builder codec.PkgBuilder, pkg []byte) (*codec.PKG, error) {
	p, err := builder.Unpack(pkg)
	if err != nil {
		return nil, err
	}
	if p.Action.Val() <= 0 {
		return nil, errors.New("action id zero")
	}

	return p, nil
}

func (e *Event) actionEncode(c socket.Conn, gwPkg *codec.PKG) ([]byte, error) {
	b, _ := c.Context().GetOptional("pkgBuilder")
	builder := b.(codec.PkgBuilder)
	return builder.Pack(gwPkg)
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

func (e *Event) cryptEnable() bool {
	return e.crypto != nil
}

func (e *Event) decrypt(c socket.Conn, pkg []byte) ([]byte, error) {
	if !e.cryptEnable() || len(pkg) == 0 {
		return pkg, nil
	}
	key1, ok := c.Context().GetOptional("cryptKey")
	if !ok {
		return nil, errors.New("no key")
	}
	key := key1.([]byte)
	index := len(pkg) - e.crypto.Type().IvLen()
	iv := pkg[index:]
	pkg = pkg[:index]
	if len(pkg) == 0 {
		return pkg, nil
	}
	return e.crypto.Decrypt(pkg, key, iv, false)
}

func (e *Event) encrypt(c socket.Conn, pkg []byte) ([]byte, []byte, error) {
	if !e.cryptEnable() || len(pkg) == 0 {
		return pkg, nil, nil
	}
	key1, ok := c.Context().GetOptional("cryptKey")
	if !ok {
		return nil, nil, errors.New("no key")
	}
	key := key1.([]byte)
	return e.crypto.Encrypt(pkg, key, false)
}

func (e *Event) gatewayWrite(c socket.Conn, action codec.Action, data []byte) error {
	// 加密
	encrypted, iv, err := e.encrypt(c, data)
	if err != nil {
		return utils.NewWrappedError("write failed, data encrypt failed", err)
	}
	// gateway包
	gwPkg := &codec.PKG{
		Action: action.Id,
		Data:   encrypted,
	}
	actionPkg, err := e.actionEncode(c, gwPkg)
	if err != nil {
		return utils.NewWrappedError("write failed, encode gateway package failed", err)
	}
	actionPkg = append(actionPkg, iv...)
	// codec
	packedPkg, err := e.codecEncode(c, actionPkg)
	if err != nil {
		return utils.NewWrappedError("write failed, codec encode failed", err)
	}
	if err = c.Write(packedPkg); err != nil {
		return utils.NewWrappedError("write failed", err)
	}
	return nil
}

func (e *Event) Send(c socket.Conn, a codec.Action, data []byte) (err error) {
	if err = e.gatewayWrite(c, a, data); err != nil {
		e.log(c, "send action failed, err="+err.Error(), zapcore.ErrorLevel, zap.String("action", a.String()), zap.ByteString("pkg", data))
	} else {
		e.log(c, "sent action["+a.String()+"]", zapcore.InfoLevel)
	}
	return
}

func (e *Event) SendAction(c socket.Conn, a codec.Action, data codec.DataPtr) (err error) {
	b := e.DataBuilder(c)
	bb, err := b.Pack(data)
	if err != nil {
		return utils.NewWrappedError("send action failed, data encode failed", err)
	}

	if err = e.gatewayWrite(c, a, bb); err != nil {
		e.log(c, "send action failed, err="+err.Error(), zapcore.ErrorLevel, zap.String("action", a.String()), zap.ByteString("pkg", bb))
	} else {
		e.log(c, "sent action["+a.String()+"]", zapcore.InfoLevel)
	}
	return
}

func (e *Event) gatewayErrorResponse(c socket.Conn, stat gatewayv1.GatewayError_Status, triggerActionId uint32) error {
	data := &gatewayv1.GatewayError{
		Status:        stat,
		TriggerAction: triggerActionId,
	}

	a := action.New(gatewayv1.ActionId_GatewayErr)
	e.log(c, "gateway response, action="+a.String(), zapcore.ErrorLevel, zap.String("action", a.String()), zap.Uint32("triggerActionId", triggerActionId))
	return e.SendAction(c, a, data)
}

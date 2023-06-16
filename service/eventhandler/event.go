package eventhandler

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/application/pkg/security"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/socketgateway/pkg/group"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/action"
	"github.com/obnahsgnaw/socketgateway/service/codec"
	gatewayv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/gateway/v1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"strconv"
	"time"
)

// Event 事件处理引擎
type Event struct {
	ctx           context.Context
	am            *action.Manager
	tickHandlers  []TickHandler
	logWatcher    func(socket.Conn, string, zapcore.Level, ...zap.Field)
	logger        *zap.Logger
	st            sockettype.SocketType
	codecProvider codec.Provider
	codedProvider codec.DataBuilderProvider
	authEnable    bool
	tickInterval  time.Duration
	crypto        *security.EsCrypto
	staticEsKey   []byte // 固定的加解密key，如果使用固定的加解密key则所有action都进行此key的加解密， 否则根据认证后的key进行加解密（认证key不进行此加解密）
}

func New(ctx context.Context, m *action.Manager, st sockettype.SocketType, options ...Option) *Event {
	s := &Event{
		ctx: ctx,
		am:  m,
		st:  st,
	}
	if s.st == sockettype.WSS {
		s.codecProvider = codec.WssProvider()
	} else {
		s.codecProvider = codec.DefaultProvider()
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
		e.logger.Info(utils.ToStr("Service[", s.Type().String(), ":", strconv.Itoa(s.Port()), "] booted"))
	}
}

func (e *Event) OnOpen(_ *socket.Server, c socket.Conn) {
	e.log(c, "Connected", zapcore.InfoLevel)
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
		r, _ := c.Context().GetOptional("close_reason")
		reason = r.(string)
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
	// 读取数据
	rawPkg, err := c.Read()
	if err != nil {
		e.log(c, "read failed, err="+err.Error(), zapcore.ErrorLevel)
		return
	}
	// 获取设置 拆包器、action解码器、数据解码器
	coderName, unpacker, pkgBuilder := e.initCodec(c, rawPkg)
	// 拆包
	err = e.codecDecode(c, unpacker, rawPkg, func(packedPkg []byte) {
		e.log(c, "package received", zapcore.DebugLevel, zap.ByteString("package", packedPkg))
		// 解码action
		gwPkg, err := e.actionDecode(pkgBuilder, packedPkg)
		if err != nil {
			e.log(c, "action decode failed, err="+err.Error(), zapcore.ErrorLevel)
			if err = e.gatewayErrorResponse(c, gatewayv1.ActionId_ActionErr, 0); err != nil {
				e.log(c, err.Error(), zapcore.ErrorLevel)
			}
			return
		}
		// 获取action
		rqAction, ok := e.am.GetAction(codec.ActionId(gwPkg.Action))
		if !ok {
			e.log(c, "action no handler, action="+gwPkg.String(), zapcore.ErrorLevel, zap.Uint32("action", gwPkg.Action))
			if err = e.gatewayErrorResponse(c, gatewayv1.ActionId_NoActionHandler, gwPkg.Action); err != nil {
				e.log(c, err.Error(), zapcore.ErrorLevel)
			}
			return
		}
		e.log(c, "handle action： "+rqAction.String(), zapcore.InfoLevel, zap.Uint32("action_id", uint32(rqAction.Id)), zap.String("action_name", rqAction.Name))
		// 认证检查， 排除认证相关action
		if !e.authCheck(c, gatewayv1.ActionId(gwPkg.Action)) {
			e.log(c, "handle failed, no auth", zapcore.ErrorLevel)
			if err = e.gatewayErrorResponse(c, gatewayv1.ActionId_NoAuth, gwPkg.Action); err != nil {
				e.log(c, err.Error(), zapcore.ErrorLevel)
			}
			return
		}
		// 解密
		decryptedData, err := e.decrypt(c, gatewayv1.ActionId(gwPkg.Action), []byte(gwPkg.Iv), gwPkg.Data)
		if err != nil {
			e.log(c, "data decrypt failed, err="+err.Error(), zapcore.ErrorLevel)
			if err = e.gatewayErrorResponse(c, gatewayv1.ActionId_DecryptErr, 0); err != nil {
				e.log(c, err.Error(), zapcore.ErrorLevel)
			}
			return
		}

		// 分发action 获得响应信息
		respAction, respData, err := e.am.Dispatch(c, coderName, e.DataBuilder(c), codec.ActionId(gwPkg.Action), decryptedData)
		if err != nil {
			if err.Error() == "NotFound" {
				e.log(c, "package dispatch failed,err="+err.Error(), zapcore.ErrorLevel)
				if err = e.gatewayErrorResponse(c, gatewayv1.ActionId_NoActionHandler, uint32(rqAction.Id)); err != nil {
					e.log(c, err.Error(), zapcore.ErrorLevel)
				}
				return
			}
			e.log(c, "package dispatch failed,err="+err.Error(), zapcore.ErrorLevel)
			if err = e.gatewayErrorResponse(c, gatewayv1.ActionId_InternalErr, uint32(rqAction.Id)); err != nil {
				e.log(c, err.Error(), zapcore.ErrorLevel)
			}
			return
		}
		// 回复
		if respAction.Id > 0 {
			e.log(c, "handle success, response action= "+respAction.String(), zapcore.InfoLevel)
			if err = e.gatewayWrite(c, respAction, respData); err != nil {
				e.log(c, "response failed,err="+err.Error(), zapcore.ErrorLevel)
			} else {
				e.log(c, "action response "+respAction.String(), zapcore.InfoLevel, zap.Uint32("action_id", uint32(rqAction.Id)), zap.String("action_name", rqAction.Name), zap.ByteString("data", respData))
			}
			return
		}
		e.log(c, "handle success, but no response", zapcore.InfoLevel)
	})
	if err != nil {
		e.log(c, "codec decode failed, err="+err.Error(), zapcore.ErrorLevel, zap.ByteString("package", rawPkg))
		if err = e.gatewayErrorResponse(c, gatewayv1.ActionId_PackageErr, 0); err != nil {
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
		e.logger.Info(utils.ToStr("Service[", s.Type().String(), ":", strconv.Itoa(s.Port()), "] down"))
	}
}

func (e *Event) log(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field) {
	if e.logger != nil {
		e.logger.Log(l, msg, data...)
	}
	if e.logWatcher != nil {
		e.logWatcher(c, msg, l, data...)
	}
}

func (e *Event) WatchLog(watcher func(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field)) {
	e.logWatcher = watcher
}

func (e *Event) AddTicker(ticker TickHandler) {
	e.tickHandlers = append(e.tickHandlers, ticker)
}

func (e *Event) initCodec(c socket.Conn, pkg []byte) (coderName codec.Name, unpacker codec.Codec, pkgBuilder codec.PkgBuilder) {
	if p, ok := c.Context().GetOptional("codec"); ok {
		unpacker = p.(codec.Codec)
		pb, _ := c.Context().GetOptional("pkgBuilder")
		n, _ := c.Context().GetOptional("coderName")
		pkgBuilder = pb.(codec.PkgBuilder)
		coderName = n.(codec.Name)
	} else {
		coderName, unpacker, pkgBuilder = e.codecProvider(pkg)
		c.Context().SetOptional("codec", unpacker)
		c.Context().SetOptional("pkgBuilder", pkgBuilder)
		c.Context().SetOptional("coderName", coderName)
		c.Context().SetOptional("dataBuilder", e.codedProvider.Provider(coderName))
		e.log(c, utils.ToStr("codec format=", coderName.String()), zapcore.InfoLevel)
	}

	return
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

func (e *Event) actionDecode(builder codec.PkgBuilder, pkg []byte) (*gatewayv1.GatewayPackage, error) {
	p, err := builder.Unpack(pkg)
	if err != nil {
		return nil, err
	}
	if p.Action <= 0 {
		return nil, errors.New("action id zero")
	}

	return p, nil
}

func (e *Event) actionEncode(c socket.Conn, gwPkg *gatewayv1.GatewayPackage) ([]byte, error) {
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

func (e *Event) cryptEnable(actionId gatewayv1.ActionId, d bool) bool {
	// 无加解密处理器
	if e.crypto == nil {
		return false
	}
	// 固定密钥 全部进行加解密
	if e.staticEsKey != nil {
		return true
	}
	// 忽略认证action
	if d && (actionId == gatewayv1.ActionId_AuthReq || actionId == gatewayv1.ActionId_Ping) {
		return false
	}
	// 忽略认证action
	if !d && (actionId == gatewayv1.ActionId_AuthResp || actionId == gatewayv1.ActionId_Pong) {
		return false
	}

	return true
}

func (e *Event) decrypt(c socket.Conn, actionId gatewayv1.ActionId, iv []byte, pkg []byte) ([]byte, error) {
	if !e.cryptEnable(actionId, true) || len(pkg) == 0 {
		return pkg, nil
	}
	key1, ok := c.Context().GetOptional("cryptKey")
	if !ok {
		return nil, errors.New("no key")
	}
	key := key1.([]byte)
	return e.crypto.Decrypt(pkg, key, iv, false)
}

func (e *Event) encrypt(c socket.Conn, actionId gatewayv1.ActionId, pkg []byte) ([]byte, []byte, error) {
	if !e.cryptEnable(actionId, false) || len(pkg) == 0 {
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
	encrypted, iv, err := e.encrypt(c, gatewayv1.ActionId(action.Id), data)
	if err != nil {
		return utils.NewWrappedError("write failed, data encrypt failed", err)
	}
	// gateway包
	gwPkg := &gatewayv1.GatewayPackage{
		Action: uint32(action.Id),
		Data:   encrypted,
		Iv:     string(iv),
	}
	actionPkg, err := e.actionEncode(c, gwPkg)
	if err != nil {
		return utils.NewWrappedError("write failed, encode gateway package failed", err)
	}
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

func (e *Event) gatewayErrorResponse(c socket.Conn, actionId gatewayv1.ActionId, triggerActionId uint32) error {
	data := &gatewayv1.GatewayErrResponse{TriggerAction: triggerActionId}

	a := action.New(actionId)
	e.log(c, "gateway response, action="+a.String(), zapcore.ErrorLevel, zap.String("action", a.String()), zap.Uint32("triggerActionId", triggerActionId))
	return e.SendAction(c, a, data)
}

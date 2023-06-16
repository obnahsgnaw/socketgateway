package client

import (
	"context"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/socketgateway/pkg/client"
	"github.com/obnahsgnaw/socketgateway/service/codec"
	gatewayv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/gateway/v1"
	"go.uber.org/zap"
	"log"
	"strconv"
	"sync"
	"time"
)

// DataStructure 提供data的数据结构
type DataStructure func() codec.DataPtr

// Handler action的处理器， rqData即DataStructure提供的数据结构解析后的结构
type Handler func(rqData codec.DataPtr) (respAction codec.Action, respData codec.DataPtr)

type Client struct {
	c        *client.Client
	handlers sync.Map
	cdc      codec.Codec
	pgb      codec.PkgBuilder
	dbd      codec.DataBuilder
	logger   *zap.Logger
}

type listenHandler struct {
	action    codec.Action
	structure DataStructure
	handler   Handler
}

func New(ctx context.Context, network string, host url.Host, cdc codec.Codec, pgb codec.PkgBuilder, dbd codec.DataBuilder, options ...Option) *Client {
	c := &Client{
		c:   client.New(ctx, network, host),
		cdc: cdc,
		pgb: pgb,
		dbd: dbd,
	}
	c.With(options...)
	c.c.With(client.Message(c.dispatch))

	return c
}

func (c *Client) With(options ...Option) {
	for _, o := range options {
		o(c)
	}
}

func (c *Client) Listen(action codec.Action, structure DataStructure, handler Handler) {
	c.addHandler(listenHandler{
		action:    action,
		structure: structure,
		handler:   handler,
	})
}

func (c *Client) Send(action codec.Action, data codec.DataPtr) error {
	b2, err := c.Pack(action, data)
	if err != nil {
		return err
	}
	err = c.c.Send(b2)
	if err != nil {
		return utils.NewWrappedError("send action["+action.Name+"] failed,send failed", err)
	}
	if c.logger != nil {
		c.logger.Debug("send action["+action.Name+"] success", zap.ByteString("pkg", b2))
	}
	return nil
}

func (c *Client) Pack(action codec.Action, data codec.DataPtr) ([]byte, error) {
	// data封包
	b, err := c.dbd.Pack(data)
	if err != nil {
		return nil, utils.NewWrappedError("send action["+action.Name+"] failed,pack data failed", err)
	}
	// todo encrypt
	// action封包
	b1, err := c.pgb.Pack(&gatewayv1.GatewayPackage{
		Action: uint32(action.Id),
		Data:   b,
		Iv:     "",
	})
	if err != nil {
		return nil, utils.NewWrappedError("send action["+action.Name+"] failed,pack gateway package failed", err)
	}
	// codec封包
	b2, err := c.cdc.Marshal(b1)
	if err != nil {
		return nil, utils.NewWrappedError("send action["+action.Name+"] failed,pack codec package failed", err)
	}

	return b2, nil
}

func (c *Client) Heartbeat(pkg []byte, interval time.Duration) {
	c.c.Heartbeat(pkg, interval)
}

func (c *Client) Start() {
	c.c.Start()
}

func (c *Client) Stop() {
	c.c.Stop()
}

func (c *Client) addHandler(handler listenHandler) {
	c.handlers.Store(handler.action.Id, handler)
}

func (c *Client) delHandler(id codec.ActionId) {
	c.handlers.Delete(id)
}

func (c *Client) getHandler(id codec.ActionId) (DataStructure, codec.Action, Handler, bool) {
	h, ok := c.handlers.Load(id)
	if ok {
		h1 := h.(listenHandler)
		return h1.structure, h1.action, h1.handler, true
	}

	return nil, codec.Action{}, nil, false
}

func (c *Client) dispatch(pkg []byte) {
	defer utils.RecoverHandler("client server dispatcher", func(err, stack string) {
		if c.logger != nil {
			c.logger.Error("dispatch failed, err=" + err + ", stack=" + stack)
		}
	})
	if c.logger != nil {
		c.logger.Debug("dispatcher: received raw package", zap.ByteString("pkg", pkg))
	}
	// 沾包拼包
	var err error
	tmp := c.c.Tmp
	c.c.Tmp = nil
	if len(tmp) > 0 {
		pkg = append(tmp, pkg...)
		if c.logger != nil {
			c.logger.Debug("dispatcher: withed tmp package", zap.ByteString("pkg", pkg))
		}
	}
	// 沾包拆包
	c.c.Tmp, err = c.cdc.Unmarshal(pkg, func(codePkg []byte) {
		if c.logger != nil {
			c.logger.Debug("dispatcher: received package", zap.ByteString("pkg", codePkg))
		}
		// 网关层的包拆包
		gatewayPackage, err1 := c.pgb.Unpack(codePkg)
		if err1 != nil {
			if c.logger != nil {
				c.logger.Error("dispatcher: unpack gateway package failed, err=" + err.Error())
			} else {
				log.Println("dispatcher error:  unpack gateway package failed, err=" + err.Error())
			}
			return
		}
		if c.logger != nil {
			c.logger.Debug("dispatcher: received action: " + strconv.Itoa(int(gatewayPackage.Action)))
		}
		// todo decrypt
		// 获取action
		ds, action, handler, ok := c.getHandler(codec.ActionId(gatewayPackage.Action))
		if !ok {
			if c.logger != nil {
				c.logger.Warn("dispatcher: no action[" + strconv.Itoa(int(gatewayPackage.Action)) + "] handler")
			} else {
				log.Println("dispatcher warn: no action[" + strconv.Itoa(int(gatewayPackage.Action)) + "] handler")
			}
			return
		}
		if c.logger != nil {
			c.logger.Info("dispatcher: handle action=" + action.String())
		}
		// data 解码
		d := ds()
		if err = c.dbd.Unpack(gatewayPackage.Data, d); err != nil {
			if c.logger != nil {
				c.logger.Error("dispatcher: action data decode failed, err=" + err.Error())
			} else {
				log.Println("dispatcher error: action data decode failed, err=" + err.Error())
			}
			return
		}
		// 处理
		respAction, respData := handler(d)
		if respAction.Id <= 0 {
			if c.logger != nil {
				c.logger.Info("dispatcher: handle success, no response")
			}
			return
		}
		if c.logger != nil {
			c.logger.Info("dispatcher: handle success, response action=" + respAction.String())
		}
		// 回复
		if err = c.Send(respAction, respData); err != nil {
			if c.logger != nil {
				c.logger.Error("dispatcher: " + err.Error())
			} else {
				log.Println("dispatcher error: " + err.Error())
			}
		}
	})
	if err != nil {
		if c.logger != nil {
			c.logger.Error("dispatcher: unpack codec package failed, err=" + err.Error())
		} else {
			log.Println("dispatcher error: unpack codec package failed, err=" + err.Error())
		}
	}
}

// todo gateway err, auth, crypt

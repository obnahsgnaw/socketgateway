package impl

import (
	"context"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/rpc/pkg/rpcclient"
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type RemoteHandler struct {
	ctx     context.Context
	tp      handlerv1.SocketType
	manager *rpcclient.Manager
}

func NewRemoteHandler(ctx context.Context, l *zap.Logger, tp handlerv1.SocketType) *RemoteHandler {
	m := rpcclient.NewManager()
	m.RegisterAfterHandler(func(ctx context.Context, head rpcclient.Header, method string, req, reply interface{}, cc *grpc.ClientConn, err error, opts ...grpc.CallOption) {
		if err != nil {
			l.Error(utils.ToStr(head.RqId, " ", head.From, " rpc call ", head.To, " socket-handler[", method, "] failed,", err.Error()), zap.Any("rq_id", head.RqId), zap.Any("req", req), zap.Any("resp", reply))
		} else {
			l.Debug(utils.ToStr(head.RqId, " ", head.From, " rpc call ", head.To, " socket-handler[", method, "] success"), zap.Any("rq_id", head.RqId), zap.Any("req", req), zap.Any("resp", reply))
		}
	})
	return &RemoteHandler{ctx: ctx, manager: m, tp: tp}
}

func (h *RemoteHandler) Call(rqId, serverHost, gateway, format string, c socket.Conn, id codec.ActionId, data []byte) (codec.Action, []byte, error) {
	h.manager.Add("handler", serverHost)
	var resp *handlerv1.HandleResponse
	err := h.manager.HostCall(h.ctx, serverHost, 1, "gateway", "handler", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		handler := handlerv1.NewHandlerServiceClient(cc)
		req := &handlerv1.HandleRequest{
			ActionId: uint32(id),
			Package:  data,
			Gateway:  gateway,
			Fd:       int64(c.Fd()),
			BindIds:  c.Context().IdMap(),
			Format:   format,
			Typ:      h.tp,
		}
		if c.Context().Authed() {
			u := c.Context().User()
			req.User = &handlerv1.HandleRequest_User{
				Id:    int64(u.Id),
				Name:  u.Name,
				Attrs: u.Attr,
			}
		}
		target := c.Context().Authentication()
		if target != nil {
			req.Target = &handlerv1.HandleRequest_Target{
				Type:     target.Type,
				Id:       target.Id,
				Master:   target.Master,
				Cid:      target.Cid,
				Uid:      target.Uid,
				Protocol: target.Protocol,
			}
		}
		var err error
		resp, err = handler.Handle(ctx, req)
		return err
	})

	if err != nil {
		return codec.Action{}, nil, utils.NewWrappedError("handler call response failed", err)
	}
	return codec.Action{
		Id:   codec.ActionId(resp.ActionId),
		Name: resp.ActionName,
	}, resp.Package, nil
}

func St2hct(st sockettype.SocketType) handlerv1.SocketType {
	switch st {
	case sockettype.TCP, sockettype.TCP4, sockettype.TCP6:
		return handlerv1.SocketType_Tcp
	case sockettype.WSS:
		return handlerv1.SocketType_Wss
	case sockettype.UDP, sockettype.UDP4, sockettype.UDP6:
		return handlerv1.SocketType_Udp
	default:
		panic("trans socket type to server type failed")
	}
}

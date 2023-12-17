package impl

import (
	"context"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/rpc/pkg/rpc"
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type RemoteHandler struct {
	manager *rpc.Manager
}

func NewRemoteHandler(l *zap.Logger) *RemoteHandler {
	m := rpc.NewManager()
	m.RegisterAfterHandler(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, err error, opts ...grpc.CallOption) {
		if err != nil {
			l.Error(utils.ToStr("rpc call socket-handler[", method, "] failed,", err.Error()), zap.Any("req", req), zap.Any("resp", reply))
		} else {
			l.Debug(utils.ToStr("rpc call socket-handler[", method, "] success"), zap.Any("req", req), zap.Any("resp", reply))
		}
	})
	return &RemoteHandler{manager: m}
}

func (h *RemoteHandler) Call(serverHost, gateway, format string, c socket.Conn, id codec.ActionId, data []byte) (codec.Action, []byte, error) {
	h.manager.Add("handler", serverHost)
	cc, err := h.manager.GetConn("handler", serverHost, 1)
	if err != nil {
		return codec.Action{}, nil, utils.NewWrappedError("handler call rpc conn failed", err)
	}
	handler := handlerv1.NewHandlerServiceClient(cc)
	req := &handlerv1.HandleRequest{
		ActionId: uint32(id),
		Package:  data,
		Gateway:  gateway,
		Fd:       int64(c.Fd()),
		BindIds:  c.Context().IdMap(),
		Format:   format,
	}
	if c.Context().Authed() {
		u := c.Context().User()
		req.User = &handlerv1.HandleRequest_User{
			Id:   int64(u.Id),
			Name: u.Name,
		}
	}
	resp, err := handler.Handle(context.Background(), req)
	if err != nil {
		return codec.Action{}, nil, utils.NewWrappedError("handler call response failed", err)
	}
	return codec.Action{
		Id:   codec.ActionId(resp.ActionId),
		Name: resp.ActionName,
	}, resp.Package, nil
}

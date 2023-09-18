package impl

import (
	"context"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/rpc/pkg/rpc"
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/codec"
)

type RemoteHandler struct {
	manager *rpc.Manager
}

func NewRemoteHandler() *RemoteHandler {
	return &RemoteHandler{manager: rpc.NewManager()}
}

func (h *RemoteHandler) Call(serverHost, gateway, format string, c socket.Conn, id codec.ActionId, data []byte) (codec.Action, []byte, error) {
	h.manager.Add("handler", serverHost)
	cc, err := h.manager.GetConn("handler", serverHost, 1)
	if err != nil {
		return codec.Action{}, nil, utils.NewWrappedError("handler call rpc conn failed", err)
	}
	handler := handlerv1.NewHandlerServiceClient(cc)
	var f handlerv1.HandleRequest_Format
	if format == codec.Json.String() {
		f = handlerv1.HandleRequest_Json
	} else {
		f = handlerv1.HandleRequest_Proto
	}
	req := &handlerv1.HandleRequest{
		ActionId: uint32(id),
		Package:  data,
		Gateway:  gateway,
		Fd:       int64(c.Fd()),
		Id:       c.Context().Id().Id,
		Type:     c.Context().Id().Type,
		Format:   f,
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

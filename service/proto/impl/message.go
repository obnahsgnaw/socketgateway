package impl

import (
	"context"
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	messagev1 "github.com/obnahsgnaw/socketapi/gen/message/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler/connutil"
	"github.com/obnahsgnaw/socketutil/codec"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type MessageService struct {
	messagev1.UnimplementedMessageServiceServer
	s func() *socket.Server
	e func() *eventhandler.Event
}

func NewMessageService(s func() *socket.Server, e func() *eventhandler.Event) *MessageService {
	return &MessageService{
		s: s,
		e: e,
	}
}

func (gw *MessageService) SendMessage(ctx context.Context, in *messagev1.SendMessageRequest) (resp *messagev1.SendMessageResponse, err error) {
	var rqId string
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		ids := md.Get("rq_id")
		if len(ids) > 0 {
			rqId = ids[0]
		}
	}

	if in.ActionId == 0 {
		err = status.New(codes.InvalidArgument, "param:ActionId is required").Err()
		return
	}
	var cc []socket.Conn
	if in.GetFd() > 0 {
		ccc := gw.s().GetFdConn(int(in.GetFd()))
		if ccc != nil {
			cc = append(cc, ccc)
		}
	} else if in.GetId() != nil {
		if in.GetId().Type == "TARGET" || in.GetId().Type == "SN" {
			ccIds := gw.s().QueryProxyTargetBinds(in.GetId().Id)
			if len(ccIds) > 0 {
				for _, ccId := range ccIds {
					ccc := gw.s().GetFdConn(ccId)
					if ccc != nil {
						cc = append(cc, ccc)
					}
				}
			}
		}
		if len(cc) == 0 {
			cc = gw.s().GetIdConn(socket.ConnId{
				Id:   in.GetId().Id,
				Type: in.GetId().Type,
			})
		}
		if len(cc) == 0 {
			err = status.New(codes.NotFound, "connection not found or not support").Err()
			return
		}
	}
	var send bool
	var lastErr error
	for _, c := range cc {
		coderName := connutil.CoderName(c)
		var msg []byte
		if c.Context().Authentication().Protocol != "" {
			var subActions []*handlerv1.SubAction
			if msg, subActions, lastErr = gw.e().ActionManager().Raw(c, rqId, gw.e().InternalDataCoder(), c.Context().Authentication().Protocol, in.PbMessage, in.ActionId); lastErr != nil {
				continue
			}
			if err = gw.e().SendRaw(c, msg); err != nil {
				lastErr = status.New(codes.Internal, "send raw message failed, err="+err.Error()).Err()
			} else {
				send = true
			}
			for _, subAction := range subActions {
				if subAction.ActionId == 0 && len(subAction.Data) > 0 {
					subConn := c
					if subAction.Target != "" {
						conns := gw.e().SocketServer().GetAuthenticatedSnConn(subAction.Target)
						if len(conns) > 0 {
							subConn = conns[0]
						} else {
							subConn = nil
						}
					}
					if subConn == nil {
						lastErr = status.New(codes.Internal, "send raw message failed, err=sub target conn not found").Err()
						continue
					}
					if err = gw.e().SendRaw(subConn, subAction.Data); err != nil {
						lastErr = status.New(codes.Internal, "send raw message failed, err="+err.Error()).Err()
					} else {
						send = true
					}
				}
			}
		} else {
			if coderName == codec.Proto {
				msg = in.PbMessage
			} else {
				msg = in.JsonMessage
			}
			if err = gw.e().Send(c, rqId, codec.NewAction(codec.ActionId(in.ActionId), in.ActionName), msg); err != nil {
				lastErr = status.New(codes.Internal, "send message failed, err="+err.Error()).Err()
			} else {
				send = true
			}
		}
	}
	// 发送到一个端即可
	if !send {
		err = lastErr
	}

	resp = &messagev1.SendMessageResponse{}
	return
}

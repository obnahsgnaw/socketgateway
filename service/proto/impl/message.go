package impl

import (
	"context"
	messagev1 "github.com/obnahsgnaw/socketapi/gen/message/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
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
	var c socket.Conn
	if in.GetFd() > 0 {
		c = gw.s().GetFdConn(int(in.GetFd()))
	} else if in.GetId() != nil {
		c = gw.s().GetIdConn(socket.ConnId{
			Id:   in.GetId().Id,
			Type: in.GetId().Type,
		})
	}
	if c == nil {
		err = status.New(codes.NotFound, "connection not found or not support").Err()
		return
	}
	n, _ := c.Context().GetOptional("coderName")
	coderName := n.(codec.Name)
	var msg []byte
	if coderName == codec.Proto {
		msg = in.PbMessage
	} else {
		msg = in.JsonMessage
	}
	if err = gw.e().Send(c, rqId, codec.NewAction(codec.ActionId(in.ActionId), in.ActionName), msg); err != nil {
		err = status.New(codes.Internal, "send message failed, err="+err.Error()).Err()
	}

	resp = &messagev1.SendMessageResponse{}
	return
}

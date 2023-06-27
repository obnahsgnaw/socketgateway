package impl

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/codec"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	messagev1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/message/v1"
)

type MessageService struct {
	messagev1.UnimplementedMessageServiceServer
	s *socket.Server
	e *eventhandler.Event
}

func NewMessageService(s *socket.Server, e *eventhandler.Event) *MessageService {
	return &MessageService{
		s: s,
		e: e,
	}
}

func (gw *MessageService) SendMessage(_ context.Context, in *messagev1.SendMessageRequest) (_ *messagev1.SendMessageResponse, err error) {
	if in.ActionId == 0 {
		err = errors.New("action id is required")
		return
	}
	if len(in.GetMessage()) == 0 {
		err = errors.New("message is required")
		return
	}
	var c socket.Conn
	if in.GetFd() > 0 {
		c = gw.s.GetFdConn(int(in.GetFd()))
	} else if in.GetId() != nil {
		c = gw.s.GetIdConn(socket.ConnId{
			Id:   in.GetId().Id,
			Type: in.GetId().Type,
		})
	}
	if c == nil {
		err = errors.New("fd conn not exists")
		return
	}
	if err = gw.e.Send(c, codec.NewAction(codec.ActionId(in.ActionId), in.ActionName), in.Message); err != nil {
		err = errors.New("send message failed, err=" + err.Error())
	}

	return
}

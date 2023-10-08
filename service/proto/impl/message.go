package impl

import (
	"context"
	"errors"
	messagev1 "github.com/obnahsgnaw/socketapi/gen/message/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	"github.com/obnahsgnaw/socketutil/codec"
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

func (gw *MessageService) SendMessage(_ context.Context, in *messagev1.SendMessageRequest) (resp *messagev1.SendMessageResponse, err error) {
	if in.ActionId == 0 {
		err = errors.New("action id is required")
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
		err = errors.New("fd conn not exists")
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
	if err = gw.e().Send(c, codec.NewAction(codec.ActionId(in.ActionId), in.ActionName), msg); err != nil {
		err = errors.New("send message failed, err=" + err.Error())
	}

	resp = &messagev1.SendMessageResponse{}
	return
}

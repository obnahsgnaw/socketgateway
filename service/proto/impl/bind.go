package impl

import (
	"context"
	"errors"
	bindv1 "github.com/obnahsgnaw/socketapi/gen/bind/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
)

type BindService struct {
	bindv1.UnimplementedBindServiceServer
	s func() *socket.Server
}

func NewBindService(s func() *socket.Server) *BindService {
	return &BindService{
		s: s,
	}
}

func (gw *BindService) BindId(_ context.Context, in *bindv1.BindIdRequest) (_ *bindv1.BindIdResponse, err error) {
	if in.Fd == 0 {
		err = errors.New("fd is required")
		return
	}
	if in.Id == "" {
		err = errors.New("id is required")
		return
	}
	if in.IdType == "" {
		err = errors.New("type is required")
		return
	}
	conn := gw.s().GetFdConn(int(in.Fd))
	if conn == nil {
		err = errors.New("fd conn not exists")
		return
	}
	gw.s().BindId(conn, socket.ConnId{
		Id:   in.Id,
		Type: in.IdType,
	})
	return
}

func (gw *BindService) BindExist(_ context.Context, in *bindv1.BindExistRequest) (p *bindv1.BindExistResponse, err error) {
	if in.Id == "" {
		err = errors.New("id is required")
		return
	}
	if in.IdType == "" {
		err = errors.New("type is required")
		return
	}
	conn := gw.s().GetIdConn(socket.ConnId{
		Id:   in.Id,
		Type: in.IdType,
	})
	p.Exist = conn == nil
	return
}

func (gw *BindService) UnBindId(_ context.Context, in *bindv1.UnBindIdRequest) (resp *bindv1.UnBindIdResponse, err error) {
	if in.Fd == 0 {
		err = errors.New("fd is required")
		return
	}
	conn := gw.s().GetFdConn(int(in.Fd))
	if conn == nil {
		err = errors.New("fd conn not exists")
		return
	}
	gw.s().UnbindId(conn, socket.ConnId{
		Id:   in.Id,
		Type: in.IdType,
	})
	resp = &bindv1.UnBindIdResponse{}
	return
}

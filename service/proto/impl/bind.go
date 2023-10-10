package impl

import (
	"context"
	bindv1 "github.com/obnahsgnaw/socketapi/gen/bind/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (gw *BindService) BindId(_ context.Context, in *bindv1.BindIdRequest) (resp *bindv1.BindIdResponse, err error) {
	if in.Fd == 0 {
		err = status.New(codes.InvalidArgument, "param:Fd is required").Err()
		return
	}
	if in.Id == "" {
		err = status.New(codes.InvalidArgument, "param:Id is required").Err()
		return
	}
	if in.IdType == "" {
		err = status.New(codes.InvalidArgument, "param:IdType is required").Err()
		return
	}
	conn := gw.s().GetFdConn(int(in.Fd))
	if conn == nil {
		err = status.New(codes.NotFound, "connection not found").Err()
		return
	}
	gw.s().BindId(conn, socket.ConnId{
		Id:   in.Id,
		Type: in.IdType,
	})
	resp = &bindv1.BindIdResponse{}
	return
}

func (gw *BindService) BindExist(_ context.Context, in *bindv1.BindExistRequest) (resp *bindv1.BindExistResponse, err error) {
	if in.Id == "" {
		err = status.New(codes.InvalidArgument, "param:Id is required").Err()
		return
	}
	if in.IdType == "" {
		err = status.New(codes.InvalidArgument, "param:IdType is required").Err()
		return
	}
	conn := gw.s().GetIdConn(socket.ConnId{
		Id:   in.Id,
		Type: in.IdType,
	})
	resp.Exist = conn == nil
	return
}

func (gw *BindService) UnBindId(_ context.Context, in *bindv1.UnBindIdRequest) (resp *bindv1.UnBindIdResponse, err error) {
	if in.Fd == 0 {
		err = status.New(codes.InvalidArgument, "param:fd is required").Err()
		return
	}
	conn := gw.s().GetFdConn(int(in.Fd))
	if conn == nil {
		err = status.New(codes.NotFound, "connection not found").Err()
		return
	}
	gw.s().UnbindId(conn, socket.ConnId{
		Id:   in.Id,
		Type: in.IdType,
	})
	resp = &bindv1.UnBindIdResponse{}
	return
}

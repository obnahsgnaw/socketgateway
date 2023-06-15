package impl

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	bindv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/bind/v1"
)

type BindService struct {
	bindv1.UnimplementedBindServiceServer
	s *socket.Server
}

func NewBindService(s *socket.Server) *BindService {
	return &BindService{
		s: s,
	}
}

func (gw *BindService) BindId(_ context.Context, in *bindv1.BindIdRequest) (_ *bindv1.BindIdResponse, err error) {
	if in.Fd == 0 {
		err = errors.New("fd is required")
		return
	}
	if in.Id.Id == "" {
		err = errors.New("id is required")
		return
	}
	if in.Id.Type == "" {
		err = errors.New("type is required")
		return
	}
	conn := gw.s.GetFdConn(int(in.Fd))
	if conn == nil {
		err = errors.New("fd conn not exists")
		return
	}
	gw.s.BindId(conn, socket.ConnId{
		Id:   in.Id.Id,
		Type: in.Id.Type,
	})
	return
}

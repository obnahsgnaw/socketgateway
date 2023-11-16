package impl

import (
	"context"
	slbv1 "github.com/obnahsgnaw/socketapi/gen/slb/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strconv"
)

type SlbService struct {
	slbv1.UnimplementedSlbServiceServer
	s func() *socket.Server
}

func NewSlbService(s func() *socket.Server) *SlbService {
	return &SlbService{
		s: s,
	}
}

func (gw *SlbService) SetActionSlb(_ context.Context, in *slbv1.ActionSlbRequest) (resp *slbv1.ActionSLbResponse, err error) {
	if in.Fd == 0 {
		err = status.New(codes.InvalidArgument, "param:Fd is required").Err()
		return
	}

	resp = &slbv1.ActionSLbResponse{
		Error: "",
	}
	if in.Action <= 0 || in.Sbl <= 0 {
		return
	}
	conn := gw.s().GetFdConn(int(in.Fd))
	if conn == nil {
		err = status.New(codes.NotFound, "connection not found").Err()
		return
	}
	conn.Context().SetOptional("flb"+strconv.Itoa(int(in.Action)), int(in.Sbl))
	return
}

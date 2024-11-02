package impl

import (
	"context"
	slbv1 "github.com/obnahsgnaw/socketapi/gen/slb/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler/connutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	resp = &slbv1.ActionSLbResponse{Error: ""}
	if in.Fd == 0 {
		return
	}

	if in.Action <= 0 || in.Sbl <= 0 {
		return
	}
	conn := gw.s().GetFdConn(int(in.Fd))
	if conn == nil {
		err = status.New(codes.NotFound, "connection not found").Err()
		return
	}
	connutil.SetActionFlb(conn, int(in.Action), int(in.Sbl))
	return
}

package impl

import (
	"context"
	connv1 "github.com/obnahsgnaw/socketapi/gen/conninfo/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ConnService struct {
	connv1.UnimplementedConnServiceServer
	s func() *socket.Server
}

func NewConnService(s func() *socket.Server) *ConnService {
	return &ConnService{
		s: s,
	}
}

func (gw *ConnService) Info(_ context.Context, in *connv1.ConnInfoRequest) (resp *connv1.ConnInfoResponse, err error) {
	if in.Fd == 0 {
		resp = &connv1.ConnInfoResponse{}
		return
	}
	conn := gw.s().GetFdConn(int(in.Fd))
	if conn == nil {
		err = status.New(codes.NotFound, "connection not found").Err()
		return
	}

	u := conn.Context().User()
	var uid uint32
	var uname string
	if u != nil {
		uid = uint32(u.Id)
		uname = u.Name
	}
	resp = &connv1.ConnInfoResponse{
		LocalNetwork:    conn.LocalAddr().Network(),
		LocalAddr:       conn.LocalAddr().String(),
		RemoteNetwork:   conn.RemoteAddr().Network(),
		RemoteAddr:      conn.RemoteAddr().String(),
		ConnectAt:       conn.Context().ConnectedAt().Format("2006-01-02 15:04:05"),
		Uid:             uid,
		Uname:           uname,
		SocketType:      gw.s().Type().String(),
		TargetType:      conn.Context().Authentication().Type,
		TargetId:        conn.Context().Authentication().Id,
		TargetCid:       conn.Context().Authentication().Cid,
		TargetUid:       conn.Context().Authentication().Uid,
		TargetSid:       conn.Context().Authentication().SessionId(),
		TargetMasterSid: conn.Context().Authentication().MasterSessionId(),
	}
	return
}

func (gw *ConnService) SessionId(_ context.Context, in *connv1.ConnSidRequest) (resp *connv1.ConnSidResponse, err error) {
	sid, _ := gw.s().GetSessionId(in.Target)
	resp = &connv1.ConnSidResponse{
		SessionId: sid,
	}
	return
}

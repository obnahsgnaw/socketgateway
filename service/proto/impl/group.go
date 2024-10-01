package impl

import (
	"context"
	groupv1 "github.com/obnahsgnaw/socketapi/gen/group/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	"github.com/obnahsgnaw/socketutil/codec"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type GroupService struct {
	groupv1.UnimplementedGroupServiceServer
	s func() *socket.Server
	e func() *eventhandler.Event
}

func NewGroupService(s func() *socket.Server, e func() *eventhandler.Event) *GroupService {
	return &GroupService{
		s: s,
		e: e,
	}
}

func (gw *GroupService) JoinGroup(_ context.Context, in *groupv1.JoinGroupRequest) (resp *groupv1.JoinGroupResponse, err error) {
	if in.GetGroup().Name == "" {
		err = status.New(codes.InvalidArgument, "param:Group.Name is required").Err()
		return
	}
	if in.GetMember().GetId() == "" {
		err = status.New(codes.InvalidArgument, "param:Member.Id is required").Err()
		return
	}
	if in.GetMember().GetFd() == 0 {
		err = status.New(codes.InvalidArgument, "param:Member.Fd is required").Err()
		return
	}
	gw.s().Groups().GetGroup(in.GetGroup().GetName()).Join(int(in.Member.GetFd()), in.Member.GetId())

	resp = &groupv1.JoinGroupResponse{}
	return
}

func (gw *GroupService) LeaveGroup(_ context.Context, in *groupv1.LeaveGroupRequest) (resp *groupv1.LeaveGroupResponse, err error) {
	if in.GetGroup().Name == "" {
		err = status.New(codes.InvalidArgument, "param:Group.Name is required").Err()
		return
	}
	if in.GetFd() == 0 {
		err = status.New(codes.InvalidArgument, "param:Fd is required").Err()
		return
	}
	gw.s().Groups().GetGroup(in.GetGroup().GetName()).Leave(int(in.GetFd()))

	resp = &groupv1.LeaveGroupResponse{}
	return
}

func (gw *GroupService) BroadcastGroup(ctx context.Context, in *groupv1.BroadcastGroupRequest) (resp *groupv1.BroadcastGroupResponse, err error) {
	var rqId string
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		ids := md.Get("rq_id")
		if len(ids) > 0 {
			rqId = ids[0]
		}
	}
	if in.GetGroup().Name == "" {
		err = status.New(codes.InvalidArgument, "param:Group.Name is required").Err()
		return
	}
	if in.ActionId == 0 {
		err = status.New(codes.InvalidArgument, "param:Action is required").Err()
		return
	}
	if len(in.GetPbMessage()) == 0 || len(in.GetJsonMessage()) == 0 {
		err = status.New(codes.InvalidArgument, "param:Message is required").Err()
		return
	}
	gw.s().Groups().GetGroup(in.GetGroup().GetName()).Broadcast(func(fd int, id string) {
		if in.Id == "" || in.Id == id {
			conn := gw.s().GetFdConn(fd)

			n, _ := conn.Context().GetOptional("coderName")
			coderName := n.(codec.Name)
			var msg []byte
			if coderName == codec.Proto {
				msg = in.PbMessage
			} else {
				msg = in.JsonMessage
			}

			_ = gw.e().Send(conn, rqId, codec.NewAction(codec.ActionId(in.ActionId), in.ActionName), msg)
		}
	})

	resp = &groupv1.BroadcastGroupResponse{}
	return
}

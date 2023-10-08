package impl

import (
	"context"
	"errors"
	groupv1 "github.com/obnahsgnaw/socketapi/gen/group/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	"github.com/obnahsgnaw/socketutil/codec"
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
		err = errors.New("group name is required")
		return
	}
	if in.GetMember().GetId() == "" {
		err = errors.New("group member to join is required")
		return
	}
	if in.GetMember().GetFd() == 0 {
		err = errors.New("group member fd to join is required")
		return
	}
	gw.s().Groups().GetGroup(in.GetGroup().GetName()).Join(int(in.Member.GetFd()), in.Member.GetId())

	resp = &groupv1.JoinGroupResponse{}
	return
}

func (gw *GroupService) LeaveGroup(_ context.Context, in *groupv1.LeaveGroupRequest) (resp *groupv1.LeaveGroupResponse, err error) {
	if in.GetGroup().Name == "" {
		err = errors.New("group name is required")
		return
	}
	if in.GetFd() == 0 {
		err = errors.New("group member fd is required")
		return
	}
	gw.s().Groups().GetGroup(in.GetGroup().GetName()).Leave(int(in.GetFd()))

	resp = &groupv1.LeaveGroupResponse{}
	return
}

func (gw *GroupService) BroadcastGroup(_ context.Context, in *groupv1.BroadcastGroupRequest) (resp *groupv1.BroadcastGroupResponse, err error) {
	if in.GetGroup().Name == "" {
		err = errors.New("group name is required")
		return
	}
	if in.ActionId == 0 {
		err = errors.New("action id is required")
		return
	}
	if len(in.GetPbMessage()) == 0 || len(in.GetJsonMessage()) == 0 {
		err = errors.New("message is required")
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

			_ = gw.e().Send(conn, codec.NewAction(codec.ActionId(in.ActionId), in.ActionName), msg)
		}
	})

	resp = &groupv1.BroadcastGroupResponse{}
	return
}

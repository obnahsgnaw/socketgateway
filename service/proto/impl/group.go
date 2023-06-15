package impl

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/service/codec"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler"
	groupv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/group/v1"
)

type GroupService struct {
	groupv1.UnimplementedGroupServiceServer
	s *socket.Server
	e *eventhandler.Event
}

func NewGroupService(s *socket.Server, e *eventhandler.Event) *GroupService {
	return &GroupService{
		s: s,
		e: e,
	}
}

func (gw *GroupService) JoinGroup(_ context.Context, in *groupv1.JoinGroupRequest) (_ *groupv1.JoinGroupResponse, err error) {
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
	gw.s.Groups().GetGroup(in.GetGroup().GetName()).Join(int(in.Member.GetFd()), in.Member.GetId())
	return
}

func (gw *GroupService) LeaveGroup(_ context.Context, in *groupv1.LeaveGroupRequest) (_ *groupv1.LeaveGroupResponse, err error) {
	if in.GetGroup().Name == "" {
		err = errors.New("group name is required")
		return
	}
	if in.GetFd() == 0 {
		err = errors.New("group member fd is required")
		return
	}
	gw.s.Groups().GetGroup(in.GetGroup().GetName()).Leave(int(in.GetFd()))
	return
}

func (gw *GroupService) BroadcastGroup(_ context.Context, in *groupv1.BroadcastGroupRequest) (_ *groupv1.BroadcastGroupResponse, err error) {
	if in.GetGroup().Name == "" {
		err = errors.New("group name is required")
		return
	}
	if in.ActionId == 0 {
		err = errors.New("action id is required")
		return
	}
	if len(in.GetMessage()) == 0 {
		err = errors.New("message is required")
		return
	}
	gw.s.Groups().GetGroup(in.GetGroup().GetName()).Broadcast(func(fd int, id string) {
		if in.Id == "" || in.Id == id {
			conn := gw.s.GetFdConn(fd)
			_ = gw.e.Send(conn, codec.NewAction(codec.ActionId(in.ActionId), in.ActionName), in.Message)
		}
	})
	return
}

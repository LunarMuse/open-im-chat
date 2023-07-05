package openim

import (
	"context"

	"github.com/OpenIMSDK/Open-IM-Server/pkg/common/config"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/common/constant"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/common/log"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/discoveryregistry"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/proto/auth"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/proto/friend"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/proto/group"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/proto/msg"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/proto/sdkws"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/proto/user"
)

func NewOpenIMClient(discov discoveryregistry.SvcDiscoveryRegistry) *OpenIMClient {
	ctx := context.Background()
	userConn, err := discov.GetConn(ctx, config.Config.RpcRegisterName.OpenImUserName)
	if err != nil {
		panic(err)
	}
	friendConn, err := discov.GetConn(ctx, config.Config.RpcRegisterName.OpenImFriendName)
	if err != nil {
		panic(err)
	}
	groupConn, err := discov.GetConn(ctx, config.Config.RpcRegisterName.OpenImGroupName)
	if err != nil {
		panic(err)
	}
	authConn, err := discov.GetConn(ctx, config.Config.RpcRegisterName.OpenImAuthName)
	if err != nil {
		panic(err)
	}
	msgConn, err := discov.GetConn(ctx, config.Config.RpcRegisterName.OpenImMsgName)
	if err != nil {
		panic(err)
	}
	return &OpenIMClient{
		msg:    msg.NewMsgClient(msgConn),
		auth:   auth.NewAuthClient(authConn),
		user:   user.NewUserClient(userConn),
		group:  group.NewGroupClient(groupConn),
		friend: friend.NewFriendClient(friendConn),
	}
}

type OpenIMClient struct {
	msg    msg.MsgClient
	auth   auth.AuthClient
	user   user.UserClient
	group  group.GroupClient
	friend friend.FriendClient
}

func (o *OpenIMClient) UpdateUser(ctx context.Context, req *user.UpdateUserInfoReq) error {
	_, err := o.user.UpdateUserInfo(ctx, req)
	return err
}

func (o *OpenIMClient) UserRegister(ctx context.Context, req *sdkws.UserInfo) error {
	_, err := o.user.UserRegister(ctx, &user.UserRegisterReq{Secret: config.Config.Secret, Users: []*sdkws.UserInfo{req}})
	return err
}

func (o *OpenIMClient) AddDefaultFriend(ctx context.Context, userID string, friendUserIDs []string) error {
	_, err := o.friend.ImportFriends(ctx, &friend.ImportFriendReq{
		OwnerUserID:   userID,
		FriendUserIDs: friendUserIDs,
	})
	return err
}

func (o *OpenIMClient) AddDefaultGroup(ctx context.Context, userID string, groupID string) error {
	_, err := o.group.InviteUserToGroup(ctx, &group.InviteUserToGroupReq{
		GroupID:        groupID,
		Reason:         "",
		InvitedUserIDs: []string{userID},
	})
	return err
}

func (o *OpenIMClient) UserToken(ctx context.Context, userID string, platformID int32) (*auth.UserTokenResp, error) {
	return o.auth.UserToken(ctx, &auth.UserTokenReq{Secret: config.Config.Secret, PlatformID: platformID, UserID: userID})
}

func (o *OpenIMClient) FindGroup(ctx context.Context, groupIDs []string) ([]*sdkws.GroupInfo, error) {
	resp, err := o.group.GetGroupsInfo(ctx, &group.GetGroupsInfoReq{GroupIDs: groupIDs})
	if err != nil {
		return nil, err
	}
	return resp.GroupInfos, nil
}

func (o *OpenIMClient) MapGroup(ctx context.Context, groupIDs []string) (map[string]*sdkws.GroupInfo, error) {
	groups, err := o.FindGroup(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	groupMap := make(map[string]*sdkws.GroupInfo)
	for i, info := range groups {
		groupMap[info.GroupID] = groups[i]
	}
	return groupMap, nil
}

func (o *OpenIMClient) ForceOffline(ctx context.Context, userID string) error {
	for id := range constant.PlatformID2Name {
		_, err := o.auth.ForceLogout(ctx, &auth.ForceLogoutReq{
			PlatformID: int32(id),
			UserID:     userID,
		})
		if err != nil {
			log.ZError(ctx, "ForceOffline", err, "userID", userID, "platformID", id)
		}
	}
	return nil
}

func (o *OpenIMClient) GetGroupMemberID(ctx context.Context, groupID string) ([]string, error) {
	resp, err := o.group.GetGroupMemberUserIDs(ctx, &group.GetGroupMemberUserIDsReq{GroupID: groupID})
	if err != nil {
		return nil, err
	}
	return resp.UserIDs, nil
}

func (o *OpenIMClient) GetFriendID(ctx context.Context, userID string) ([]string, error) {
	resp, err := o.friend.GetFriendIDs(ctx, &friend.GetFriendIDsReq{UserID: userID})
	if err != nil {
		return nil, err
	}
	return resp.FriendIDs, nil
}

func (o *OpenIMClient) SendMsg(ctx context.Context, req *msg.SendMsgReq) (*msg.SendMsgResp, error) {
	return o.msg.SendMsg(ctx, req)
}

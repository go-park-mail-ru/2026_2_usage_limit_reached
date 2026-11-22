package grpc

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newGetUsersResponse(users []*dto.UserInfo) *identitypb.GetUsersResponse {
	resp := &identitypb.GetUsersResponse{Users: make([]*identitypb.UserInfo, 0, len(users))}
	for _, user := range users {
		resp.Users = append(resp.Users, newUserInfo(user))
	}

	return resp
}

func newUserInfo(user *dto.UserInfo) *identitypb.UserInfo {
	info := &identitypb.UserInfo{
		Id:        user.ID.String(),
		Username:  user.Username,
		Nickname:  user.Nickname,
		AvatarUrl: stringValue(user.AvatarURL),
		Status:    user.Status,
	}
	if !user.CreatedAt.IsZero() {
		info.CreatedAt = timestamppb.New(user.CreatedAt)
	}

	return info
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

package grpc

import (
	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/dto"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/dto"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newGetUsersResponse(users []*userdto.UserInfo) *identitypb.GetUsersResponse {
	resp := &identitypb.GetUsersResponse{Users: make([]*identitypb.UserInfo, 0, len(users))}
	for _, user := range users {
		resp.Users = append(resp.Users, newUserInfo(user))
	}

	return resp
}

func newUserInfo(user *userdto.UserInfo) *identitypb.UserInfo {
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

func newGetAuthorsResponse(authors []*authordto.AuthorInfo) *identitypb.GetAuthorsResponse {
	resp := &identitypb.GetAuthorsResponse{Authors: make([]*identitypb.Author, 0, len(authors))}
	for _, author := range authors {
		resp.Authors = append(resp.Authors, &identitypb.Author{
			Id:       author.ID.String(),
			Bio:      author.Bio,
			Category: author.Category,
		})
	}

	return resp
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

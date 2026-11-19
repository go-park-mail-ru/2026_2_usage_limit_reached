package identityclient

import (
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/dto"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toAuthResult(resp *identitypb.AuthResponse) *dto.AuthResult {
	return &dto.AuthResult{
		User:      toUserResponse(resp.GetUser()),
		Token:     resp.GetAccessToken(),
		ExpiresIn: time.Duration(resp.GetExpiresIn()) * time.Second,
	}
}

func toUserResponse(user *identitypb.UserInfo) dto.UserResponse {
	if user == nil {
		return dto.UserResponse{}
	}

	return dto.UserResponse{
		Username:  user.GetUsername(),
		Nickname:  user.GetNickname(),
		AvatarURL: stringPtr(user.GetAvatarUrl()),
	}
}

func toProfileResponse(profile *identitypb.MyProfile) (*dto.ProfileResponse, error) {
	user, err := toProfileUser(profile.GetUser(), profile.GetEmail())
	if err != nil {
		return nil, err
	}

	return &dto.ProfileResponse{User: user, Author: toAuthorResponse(profile.GetAuthor())}, nil
}

func toPublicProfileResponse(profile *identitypb.PublicProfile) (*dto.PublicProfileResponse, error) {
	user, err := toPublicProfileUser(profile.GetUser())
	if err != nil {
		return nil, err
	}

	return &dto.PublicProfileResponse{User: user, Author: toAuthorResponse(profile.GetAuthor())}, nil
}

func toProfileUser(user *identitypb.UserInfo, email string) (dto.ProfileUserResponse, error) {
	publicUser, err := toPublicProfileUser(user)
	if err != nil {
		return dto.ProfileUserResponse{}, err
	}

	return dto.ProfileUserResponse{PublicProfileUserResponse: publicUser, Email: email}, nil
}

func toPublicProfileUser(user *identitypb.UserInfo) (dto.PublicProfileUserResponse, error) {
	id, err := uuid.Parse(user.GetId())
	if err != nil {
		return dto.PublicProfileUserResponse{}, fmt.Errorf("parse user id: %w", err)
	}

	return dto.PublicProfileUserResponse{
		ID:        id,
		Username:  user.GetUsername(),
		Nickname:  user.GetNickname(),
		AvatarURL: stringPtr(user.GetAvatarUrl()),
		CreatedAt: timestampTime(user.GetCreatedAt()),
	}, nil
}

func toAuthorResponse(author *identitypb.Author) *dto.AuthorResponse {
	if author == nil {
		return nil
	}

	return &dto.AuthorResponse{Bio: author.GetBio(), Category: author.Category}
}

func timestampTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}

	return ts.AsTime()
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

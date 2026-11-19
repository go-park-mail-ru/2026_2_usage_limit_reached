package grpc

import (
	"time"

	authdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/dto"
	profiledto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/dto"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newRegistrationRequest(req *identitypb.RegisterRequest) authdto.RegistrationRequest {
	return authdto.RegistrationRequest{Email: req.GetEmail(), Username: req.GetUsername(), Nickname: req.GetNickname(), Password: req.GetPassword()}
}

func newLoginRequest(req *identitypb.LoginRequest) authdto.LoginRequest {
	return authdto.LoginRequest{Login: req.GetLogin(), Password: req.GetPassword()}
}

func newUpdateProfileRequest(req *identitypb.UpdateMyProfileRequest) profiledto.UpdateProfileRequest {
	update := profiledto.UpdateProfileRequest{Nickname: req.GetNickname()}
	if req.GetAuthor() != nil {
		update.Author = &profiledto.UpdateAuthorRequest{Bio: req.GetAuthor().GetBio(), Category: req.GetAuthor().Category}
	}
	return update
}

func newBecomeAuthorRequest(req *identitypb.BecomeAuthorRequest) profiledto.UpdateAuthorRequest {
	return profiledto.UpdateAuthorRequest{Bio: req.GetBio(), Category: req.Category}
}

func newAuthResponse(user *authdto.UserResponse, token string, ttl time.Duration) *identitypb.AuthResponse {
	return &identitypb.AuthResponse{
		AccessToken: token,
		ExpiresIn:   int64(ttl.Seconds()),
		User:        newUserInfo("", user.Username, user.Nickname, user.AvatarURL, time.Time{}),
	}
}

func newMyProfile(profile *profiledto.ProfileResponse) *identitypb.MyProfile {
	return &identitypb.MyProfile{
		User:   newUserInfo(profile.User.ID.String(), profile.User.Username, profile.User.Nickname, profile.User.AvatarURL, profile.User.CreatedAt),
		Email:  profile.User.Email,
		Author: newProfileAuthor(profile.Author, profile.User.ID.String()),
	}
}

func newPublicProfile(profile *profiledto.PublicProfileResponse) *identitypb.PublicProfile {
	return &identitypb.PublicProfile{
		User:   newUserInfo(profile.User.ID.String(), profile.User.Username, profile.User.Nickname, profile.User.AvatarURL, profile.User.CreatedAt),
		Author: newProfileAuthor(profile.Author, profile.User.ID.String()),
	}
}

func newUserInfo(id, username, nickname string, avatarURL *string, createdAt time.Time) *identitypb.UserInfo {
	card := &identitypb.UserInfo{
		Id:        id,
		Username:  username,
		Nickname:  nickname,
		AvatarUrl: stringValue(avatarURL),
	}
	if !createdAt.IsZero() {
		card.CreatedAt = timestamppb.New(createdAt)
	}
	return card
}

func newProfileAuthor(author *profiledto.AuthorResponse, id string) *identitypb.Author {
	if author == nil {
		return nil
	}

	return &identitypb.Author{Id: id, Bio: author.Bio, Category: author.Category}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

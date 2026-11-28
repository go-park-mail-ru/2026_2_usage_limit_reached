package dto

import (
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/google/uuid"
)

type ProfileUserResponse struct {
	PublicProfileUserResponse
	Email string `json:"email" example:"user@example.com"`
}

type ProfileResponse struct {
	User   ProfileUserResponse `json:"user"`
	Author *AuthorMeResponse   `json:"author,omitempty"`
}

type PublicProfileUserResponse struct {
	ID        uuid.UUID `json:"id" example:"00000000-0000-4000-8000-000000000001"`
	Username  string    `json:"username" example:"username123"`
	Nickname  string    `json:"nickname" example:"nickname_123"`
	AvatarURL *string   `json:"avatar_url,omitempty" example:"/media/avatars/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000201.png"`
	CreatedAt time.Time `json:"created_at" example:"2026-08-01T12:00:00Z"`
}

type PublicProfileResponse struct {
	User   PublicProfileUserResponse `json:"user"`
	Author *AuthorMeResponse         `json:"author,omitempty"`
}

type UpdateProfileRequest struct {
	Nickname string `json:"nickname" example:"nickname_123"`
}

func (r UpdateProfileRequest) Validate(v *validator.Validator) error {
	if !v.IsValidNickname(r.Nickname) {
		return fmt.Errorf("%w: invalid nickname", validator.ErrValidation)
	}

	return nil
}

type AvatarResponse struct {
	AvatarURL string `json:"avatar_url" example:"/media/avatars/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000201.png"`
}

func ToProfileResponse(user *models.User, avatarURL *string, author *AuthorMeResponse) *ProfileResponse {
	if user == nil {
		return nil
	}

	return &ProfileResponse{
		User: ProfileUserResponse{
			PublicProfileUserResponse: toPublicUserResponse(user, avatarURL),
			Email:                     user.Email,
		},
		Author: author,
	}
}

func ToPublicProfileResponse(user *models.User, avatarURL *string, author *AuthorMeResponse) *PublicProfileResponse {
	if user == nil {
		return nil
	}

	return &PublicProfileResponse{User: toPublicUserResponse(user, avatarURL), Author: author}
}

func toPublicUserResponse(user *models.User, avatarURL *string) PublicProfileUserResponse {
	return PublicProfileUserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		AvatarURL: avatarURL,
		CreatedAt: user.CreatedAt,
	}
}

func ToAvatarResponse(avatarURL string) *AvatarResponse {
	return &AvatarResponse{AvatarURL: avatarURL}
}

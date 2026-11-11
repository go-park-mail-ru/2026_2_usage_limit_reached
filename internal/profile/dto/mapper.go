package dto

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
)

func ToProfileResponse(profile *models.Profile, avatarURL *string) *ProfileResponse {
	if profile == nil {
		return nil
	}
	return &ProfileResponse{
		User:   toUserResponse(profile, avatarURL),
		Author: toAuthorResponse(profile.Author),
	}
}

func ToPublicProfileResponse(profile *models.Profile, avatarURL *string) *PublicProfileResponse {
	if profile == nil {
		return nil
	}

	return &PublicProfileResponse{
		User:   toPublicUserResponse(profile, avatarURL),
		Author: toAuthorResponse(profile.Author),
	}
}

func ToAvatarResponse(avatarURL string) *AvatarResponse {
	return &AvatarResponse{AvatarURL: avatarURL}
}

func toUserResponse(profile *models.Profile, avatarURL *string) ProfileUserResponse {
	return ProfileUserResponse{
		PublicProfileUserResponse: toPublicUserResponse(profile, avatarURL),
		Email:                     profile.Email,
	}
}

func toPublicUserResponse(profile *models.Profile, avatarURL *string) PublicProfileUserResponse {
	return PublicProfileUserResponse{
		ID:        profile.UserID,
		Username:  profile.Username,
		Nickname:  profile.Nickname,
		AvatarURL: avatarURL,
		CreatedAt: profile.CreatedAt,
	}
}

func toAuthorResponse(author *models.ProfileAuthor) *AuthorResponse {
	if author == nil {
		return nil
	}
	return &AuthorResponse{Bio: author.Bio, Category: author.Category}
}

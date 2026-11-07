package dto

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
)

func ToProfileResponse(profile *models.Profile) *ProfileResponse {
	if profile == nil {
		return nil
	}
	return &ProfileResponse{
		User:   toUserResponse(profile),
		Author: toAuthorResponse(profile.Author),
	}
}

func ToPublicProfileResponse(profile *models.Profile) *PublicProfileResponse {
	if profile == nil {
		return nil
	}

	return &PublicProfileResponse{
		User:   toPublicUserResponse(profile),
		Author: toAuthorResponse(profile.Author),
	}
}

func toUserResponse(profile *models.Profile) ProfileUserResponse {
	return ProfileUserResponse{
		PublicProfileUserResponse: toPublicUserResponse(profile),
		Email:                     profile.Email,
	}
}

func toPublicUserResponse(profile *models.Profile) PublicProfileUserResponse {
	return PublicProfileUserResponse{
		ID:        profile.UserID,
		Username:  profile.Username,
		Nickname:  profile.Nickname,
		CreatedAt: profile.CreatedAt,
	}
}

func toAuthorResponse(author *models.ProfileAuthor) *AuthorResponse {
	if author == nil {
		return nil
	}
	return &AuthorResponse{Bio: author.Bio, Category: author.Category}
}

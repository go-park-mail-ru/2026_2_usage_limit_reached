package dto

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/domain"
)

func ToProfileResponse(profile domain.Profile) ProfileResponse {
	posts := make([]PostResponse, 0, len(profile.Posts))
	for _, post := range profile.Posts {
		posts = append(posts, toPostResponse(post))
	}
	return ProfileResponse{
		User:   toUserResponse(profile.User),
		Author: toAuthorResponse(profile.Author),
		Posts:  posts,
	}
}

func toUserResponse(user models.User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Email:     user.Email,
		AvatarKey: user.AvatarKey,
		CreatedAt: user.CreatedAt,
	}
}

func toAuthorResponse(author *domain.Author) *AuthorResponse {
	if author == nil {
		return nil
	}
	return &AuthorResponse{Bio: author.Bio, Category: author.Category}
}

func toPostResponse(post domain.Post) PostResponse {
	return PostResponse{
		ID:          post.ID,
		Title:       post.Title,
		Body:        post.Body,
		Status:      string(post.Status),
		PublishedAt: post.PublishedAt,
	}
}

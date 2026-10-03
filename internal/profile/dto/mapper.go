package dto

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	"github.com/google/uuid"
)

func ToProfileResponse(user *models.ProfileUser, author *models.Author) *ProfileResponse {

	return &ProfileResponse{
		User:   toUserResponse(*user),
		Author: toAuthorResponse(author),
	}
}

func ToProfilePostsResponse(userID uuid.UUID, posts []models.Post) *ProfilePostsResponse {
	postResponses := make([]PostResponse, 0, len(posts))
	for _, post := range posts {
		postResponses = append(postResponses, toPostResponse(post))
	}

	return &ProfilePostsResponse{
		UserID: userID,
		Posts:  postResponses,
	}
}

func toUserResponse(user models.ProfileUser) ProfileUserResponse {
	return ProfileUserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Email:     user.Email,
		AvatarKey: user.AvatarKey,
		CreatedAt: user.CreatedAt,
	}
}

func toAuthorResponse(author *models.Author) *AuthorResponse {
	if author == nil {
		return nil
	}
	return &AuthorResponse{Bio: author.Bio, Category: author.Category}
}

func toPostResponse(post models.Post) PostResponse {
	return PostResponse{
		ID:          post.ID,
		Title:       post.Title,
		Body:        post.Body,
		Status:      string(post.Status),
		PublishedAt: post.PublishedAt,
	}
}

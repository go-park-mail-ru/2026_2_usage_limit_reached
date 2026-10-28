package dto

import (
	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	postdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	"github.com/google/uuid"
)

func ToProfileResponse(user *userdto.UserInfo, author *authordto.AuthorInfo) *ProfileResponse {

	return &ProfileResponse{
		User:   toUserResponse(*user),
		Author: toAuthorResponse(author),
	}
}

func ToProfilePostsResponse(userID uuid.UUID, posts []*postdto.PostInfo) *ProfilePostsResponse {
	postResponses := make([]PostResponse, 0, len(posts))
	for _, post := range posts {
		postResponses = append(postResponses, toPostResponse(post))
	}

	return &ProfilePostsResponse{
		UserID: userID,
		Posts:  postResponses,
	}
}

func toUserResponse(user userdto.UserInfo) ProfileUserResponse {
	return ProfileUserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Email:     user.Email,
		AvatarKey: user.AvatarKey,
		CreatedAt: user.CreatedAt,
	}
}

func toAuthorResponse(author *authordto.AuthorInfo) *AuthorResponse {
	if author == nil {
		return nil
	}
	var category string
	if author.Category != nil {
		category = *author.Category
	}
	return &AuthorResponse{Bio: author.Bio, Category: category}
}

func toPostResponse(post *postdto.PostInfo) PostResponse {
	return PostResponse{
		ID:          post.ID,
		Title:       post.Title,
		Body:        post.Body,
		Status:      post.Status,
		PublishedAt: post.PublishedAt,
	}
}

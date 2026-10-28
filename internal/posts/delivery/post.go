package posthandlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/delivery/cursor"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	sortByNew     = "new"
	sortByPopular = "popular"
	defaultLimit  = 20
	maxLimit      = 50
)

func (h *PostHandler) Feed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		h.handleError(ctx, w, "feed", "invalid limit", err)
		return
	}
	rawCursor := r.URL.Query().Get("cursor")
	userID, _ := middleware.UserIDFromContext(ctx)

	var resp dto.PostsPageResponse
	switch r.URL.Query().Get("sort") {
	case sortByNew:
		resp, err = h.feedNew(ctx, userID, rawCursor, limit)
	case sortByPopular:
		resp, err = h.feedPopular(ctx, userID, rawCursor, limit)
	default:
		err = validator.ErrValidation
	}
	if err != nil {
		h.handleError(ctx, w, "feed", "get feed", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return defaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 || limit > maxLimit {
		return 0, validator.ErrValidation
	}
	return limit, nil
}

func (h *PostHandler) feedNew(ctx context.Context, userID uuid.UUID, rawCursor string, limit int) (dto.PostsPageResponse, error) {
	cur, err := cursor.NewPosts.Decode(rawCursor)
	if err != nil {
		return dto.PostsPageResponse{}, fmt.Errorf("decode cursor: %w: %w", validator.ErrValidation, err)
	}
	page, err := h.uc.GetNewPosts(ctx, userID, cur, limit)
	if err != nil {
		return dto.PostsPageResponse{}, err
	}
	next, err := cursor.NewPosts.Encode(page.Next)
	if err != nil {
		return dto.PostsPageResponse{}, err
	}
	return dto.PostsPageResponse{Items: page.Items, NextCursor: next}, nil
}

func (h *PostHandler) feedPopular(ctx context.Context, userID uuid.UUID, rawCursor string, limit int) (dto.PostsPageResponse, error) {
	cur, err := cursor.PopularPosts.Decode(rawCursor)
	if err != nil {
		return dto.PostsPageResponse{}, fmt.Errorf("decode cursor: %w: %w", validator.ErrValidation, err)
	}
	page, err := h.uc.GetPopularPosts(ctx, userID, cur, limit)
	if err != nil {
		return dto.PostsPageResponse{}, err
	}
	next, err := cursor.PopularPosts.Encode(page.Next)
	if err != nil {
		return dto.PostsPageResponse{}, err
	}
	return dto.PostsPageResponse{Items: page.Items, NextCursor: next}, nil
}

func (h *PostHandler) ViewPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, err := getPostID(r)
	if err != nil {
		h.handleError(ctx, w, "view post", "post ID invalid", fmt.Errorf("invalid post id: %w: %w", validator.ErrValidation, err))
		return
	}

	userID, _ := middleware.UserIDFromContext(ctx)
	post, err := h.uc.GetPost(ctx, userID, postID)
	if err != nil {
		h.handleError(ctx, w, "view post", "get post error", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, post)
}

func (h *PostHandler) NewPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer r.Body.Close()
	var postReq dto.PostRequest
	if err := response.DecodeJSON(w, r, &postReq); err != nil {
		h.handleError(ctx, w, "new post", "request body decode error", fmt.Errorf("invalid request body: %w: %w", validator.ErrValidation, err))
		return
	}
	if err := postReq.Validate(h.validator); err != nil {
		h.handleError(ctx, w, "new post", "validation error", err)
		return
	}

	userID, _ := middleware.UserIDFromContext(ctx)
	if err := h.uc.CreatePost(ctx, userID, postReq); err != nil {
		h.handleError(ctx, w, "new post", "create post error", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *PostHandler) EditPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, err := getPostID(r)
	if err != nil {
		h.handleError(ctx, w, "edit post", "post ID invalid", fmt.Errorf("invalid post id: %w: %w", validator.ErrValidation, err))
		return
	}
	defer r.Body.Close()
	var postReq dto.PostRequest
	if err := response.DecodeJSON(w, r, &postReq); err != nil {
		h.handleError(ctx, w, "edit post", "request body decode error", fmt.Errorf("invalid request body: %w: %w", validator.ErrValidation, err))
		return
	}
	if err := postReq.Validate(h.validator); err != nil {
		h.handleError(ctx, w, "edit post", "validation error", err)
		return
	}
	if postReq.LastUpdatedAt == nil {
		h.handleError(ctx, w, "edit post", "invalid last_updated_at", validator.ErrValidation)
		return
	}

	userID, _ := middleware.UserIDFromContext(ctx)
	if err := h.uc.UpdatePost(ctx, userID, postID, postReq, *postReq.LastUpdatedAt); err != nil {
		h.handleError(ctx, w, "edit post", "update post error", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, err := getPostID(r)
	if err != nil {
		h.handleError(ctx, w, "delete post", "post ID invalid", fmt.Errorf("invalid post id: %w: %w", validator.ErrValidation, err))
		return
	}

	userID, _ := middleware.UserIDFromContext(ctx)
	if err := h.uc.DeletePost(ctx, userID, postID); err != nil {
		h.handleError(ctx, w, "delete post", "delete post error", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *PostHandler) LikePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, err := getPostID(r)
	if err != nil {
		h.handleError(ctx, w, "like post", "post ID invalid", fmt.Errorf("invalid post id: %w: %w", validator.ErrValidation, err))
		return
	}

	userID, _ := middleware.UserIDFromContext(ctx)
	likesCount, err := h.uc.LikePost(ctx, userID, postID)
	if err != nil {
		h.handleError(ctx, w, "like post", "like post error", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, dto.PostLikeResponse{LikesCount: likesCount, LikedByUser: true})
}

func (h *PostHandler) UnlikePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, err := getPostID(r)
	if err != nil {
		h.handleError(ctx, w, "unlike post", "post ID invalid", fmt.Errorf("invalid post id: %w: %w", validator.ErrValidation, err))
		return
	}

	userID, _ := middleware.UserIDFromContext(ctx)
	likesCount, err := h.uc.DeletePostLike(ctx, userID, postID)
	if err != nil {
		h.handleError(ctx, w, "unlike post", "like post error", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, dto.PostLikeResponse{LikesCount: likesCount, LikedByUser: false})
}

func getPostID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

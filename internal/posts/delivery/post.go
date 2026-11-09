package posthandlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	sortByNew     = "new"
	sortByOld     = "old"
	sortByPopular = "popular"
	sortField     = "sort"
	limitField    = "limit"
	cursorFiled   = "cursor"
	defaultLimit  = 20
	maxLimit      = 50
)

type (
	getTimedPostsFunc       func(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error)
	getPopularPostsFunc     func(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error)
	getUserTimedPostsFunc   func(ctx context.Context, userID *uuid.UUID, username string, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error)
	getUserPopularPostsFunc func(ctx context.Context, userID *uuid.UUID, username string, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error)
)

// Feed возвращает ленту опубликованных постов
// @Summary Лента постов
// @Description Возвращает опубликованные посты страницами. Авторизация необязательна: для авторизованного пользователя заполняется liked_by_user. У закрытых постов (locked = true) body_preview пустой. Чтобы получить следующую страницу, передайте next_cursor из ответа в параметр cursor; значение null означает, что страниц больше нет.
// @Tags posts
// @Produce json
// @Param sort query string true "Порядок сортировки" Enums(new, old, popular)
// @Param limit query int false "Размер страницы (значения вне диапазона 1-50 заменяются на 20)" default(20) minimum(1) maximum(50)
// @Param cursor query string false "Курсор из next_cursor предыдущего ответа (для первой страницы не передаётся)"
// @Success 200 {object} dto.PostsPageResponse "Страница постов"
// @Failure 400 {object} response.ErrorResponse "Неверный sort или cursor"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /feed [get]
func (h *PostHandler) Feed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := feed.ParseLimit(r.URL.Query().Get(limitField), defaultLimit, maxLimit)
	rawCursor := r.URL.Query().Get(cursorFiled)

	var (
		resp *dto.PostsPageResponse
		err  error
	)
	userID := h.getUserIDFromContext(ctx)
	sort := r.URL.Query().Get(sortField)
	switch sort {
	case sortByNew:
		resp, err = h.feedTimed(ctx, userID, rawCursor, limit, h.uc.GetNewPosts)
	case sortByOld:
		resp, err = h.feedTimed(ctx, userID, rawCursor, limit, h.uc.GetOldPosts)
	case sortByPopular:
		resp, err = h.feedPopular(ctx, userID, rawCursor, limit, h.uc.GetPopularPosts)
	default:
		err = validator.ErrValidation
	}
	if err != nil {
		h.handleError(ctx, w, "feed", "get feed", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

// UserPosts возвращает ленту опубликованных постов конкретного автора
// @Summary Лента постов автора
// @Description Возвращает опубликованные автора посты страницами. Авторизация необязательна: для авторизованного пользователя заполняется liked_by_user. У закрытых постов (locked = true) body_preview пустой. Чтобы получить следующую страницу, передайте next_cursor из ответа в параметр cursor; значение null означает, что страниц больше нет.
// @Tags posts
// @Produce json
// @Param sort query string true "Порядок сортировки" Enums(new, old, popular)
// @Param limit query int false "Размер страницы (значения вне диапазона 1-50 заменяются на 20)" default(20) minimum(1) maximum(50)
// @Param cursor query string false "Курсор из next_cursor предыдущего ответа (для первой страницы не передаётся)"
// @Success 200 {object} dto.PostsPageResponse "Страница постов"
// @Failure 400 {object} response.ErrorResponse "Неверный sort или cursor"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /users/{username}/posts [get]
func (h *PostHandler) UserPosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	username := mux.Vars(r)["username"]
	if username == "" {
		h.handleError(ctx, w, "user posts", "username is required", validator.ErrValidation)
		return
	}

	limit := feed.ParseLimit(r.URL.Query().Get("limit"), defaultLimit, maxLimit)
	rawCursor := r.URL.Query().Get("cursor")

	var (
		resp *dto.PostsPageResponse
		err  error
	)
	userID := h.getUserIDFromContext(ctx)
	sort := r.URL.Query().Get(sortField)
	switch sort {
	case sortByNew:
		resp, err = h.userPostsTimed(ctx, userID, username, rawCursor, limit, h.uc.GetUserNewPosts)
	case sortByOld:
		resp, err = h.userPostsTimed(ctx, userID, username, rawCursor, limit, h.uc.GetUserOldPosts)
	case sortByPopular:
		resp, err = h.userPostsPopular(ctx, userID, username, rawCursor, limit, h.uc.GetUserPopularPosts)
	default:
		err = validator.ErrValidation
	}

	if err != nil {
		h.handleError(ctx, w, "user posts", "get user posts", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *PostHandler) userPostsTimed(
	ctx context.Context,
	userID *uuid.UUID,
	username string,
	rawCursor string,
	limit int,
	method getUserTimedPostsFunc,
) (*dto.PostsPageResponse, error) {
	cur, err := TimedPostsCursor.Decode(rawCursor)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w: %w", validator.ErrValidation, err)
	}

	page, err := method(ctx, userID, username, cur, limit)
	if err != nil {
		return nil, err
	}

	next, err := TimedPostsCursor.Encode(page.Next)
	if err != nil {
		return nil, err
	}

	return &dto.PostsPageResponse{Items: page.Items, NextCursor: next}, nil
}

func (h *PostHandler) userPostsPopular(
	ctx context.Context,
	userID *uuid.UUID,
	username string,
	rawCursor string,
	limit int,
	method getUserPopularPostsFunc,
) (*dto.PostsPageResponse, error) {
	cur, err := PopularPostsCursor.Decode(rawCursor)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w: %w", validator.ErrValidation, err)
	}

	page, err := method(ctx, userID, username, cur, limit)
	if err != nil {
		return nil, err
	}

	next, err := PopularPostsCursor.Encode(page.Next)
	if err != nil {
		return nil, err
	}

	return &dto.PostsPageResponse{Items: page.Items, NextCursor: next}, nil
}

// MyPosts возвращает посты текущего пользователя
// @Summary Посты текущего пользователя
// @Description Возвращает опубликованные посты авторизованного пользователя страницами. Черновики доступны через /profile/me/drafts.
// @Tags posts
// @Produce json
// @Security CookieAuth
// @Param sort query string true "Порядок сортировки" Enums(new, popular)
// @Param limit query int false "Размер страницы (значения вне диапазона 1-50 заменяются на 20)" default(20) minimum(1) maximum(50)
// @Param cursor query string false "Курсор из next_cursor предыдущего ответа (для первой страницы не передаётся)"
// @Success 200 {object} dto.PostsPageResponse "Страница постов"
// @Failure 400 {object} response.ErrorResponse "Неверный sort или cursor"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/posts [get]
func (h *PostHandler) MyPosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := feed.ParseLimit(r.URL.Query().Get(limitField), defaultLimit, maxLimit)
	rawCursor := r.URL.Query().Get(cursorFiled)

	var (
		resp *dto.PostsPageResponse
		err  error
	)
	userID := h.getUserIDFromContext(ctx)
	sort := r.URL.Query().Get(sortField)
	switch sort {
	case sortByNew:
		resp, err = h.feedTimed(ctx, userID, rawCursor, limit, h.uc.GetMyNewPosts)
	case sortByOld:
		resp, err = h.feedTimed(ctx, userID, rawCursor, limit, h.uc.GetMyOldPosts)
	case sortByPopular:
		resp, err = h.feedPopular(ctx, userID, rawCursor, limit, h.uc.GetMyPopularPosts)
	default:
		err = validator.ErrValidation
	}
	if err != nil {
		h.handleError(ctx, w, "my posts", "get my posts", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

// MyDrafts возвращает черновики текущего пользователя
// @Summary Черновики текущего пользователя
// @Description Возвращает черновики авторизованного пользователя страницами. Поддерживается только sort=new.
// @Tags posts
// @Produce json
// @Security CookieAuth
// @Param sort query string true "Порядок сортировки (поддерживается только new)" Enums(new)
// @Param limit query int false "Размер страницы (значения вне диапазона 1-50 заменяются на 20)" default(20) minimum(1) maximum(50)
// @Param cursor query string false "Курсор из next_cursor предыдущего ответа (для первой страницы не передаётся)"
// @Success 200 {object} dto.PostsPageResponse "Страница черновиков"
// @Failure 400 {object} response.ErrorResponse "Неверный sort или cursor"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/drafts [get]
func (h *PostHandler) MyDrafts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := feed.ParseLimit(r.URL.Query().Get(limitField), defaultLimit, maxLimit)
	rawCursor := r.URL.Query().Get(cursorFiled)

	var (
		resp *dto.PostsPageResponse
		err  error
	)
	userID := h.getUserIDFromContext(ctx)
	switch r.URL.Query().Get(sortField) {
	case sortByNew:
		resp, err = h.feedTimed(ctx, userID, rawCursor, limit, h.uc.GetMyNewDrafts)
	case sortByOld:
		resp, err = h.feedTimed(ctx, userID, rawCursor, limit, h.uc.GetMyOldDrafts)
	default:
		err = fmt.Errorf("invalid sort field: %w", validator.ErrValidation)
	}
	if err != nil {
		h.handleError(ctx, w, "my drafts", "get my drafts", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *PostHandler) feedTimed(
	ctx context.Context,
	userID *uuid.UUID,
	rawCursor string,
	limit int,
	getTimedPosts getTimedPostsFunc) (*dto.PostsPageResponse, error) {
	cur, err := TimedPostsCursor.Decode(rawCursor)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w: %w", validator.ErrValidation, err)
	}
	page, err := getTimedPosts(ctx, userID, cur, limit)
	if err != nil {
		return nil, err
	}
	next, err := TimedPostsCursor.Encode(page.Next)
	if err != nil {
		return nil, err
	}
	return &dto.PostsPageResponse{Items: page.Items, NextCursor: next}, nil
}

func (h *PostHandler) feedPopular(ctx context.Context,
	userID *uuid.UUID,
	rawCursor string,
	limit int,
	getPopularPosts getPopularPostsFunc) (*dto.PostsPageResponse, error) {
	cur, err := PopularPostsCursor.Decode(rawCursor)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w: %w", validator.ErrValidation, err)
	}
	page, err := getPopularPosts(ctx, userID, cur, limit)
	if err != nil {
		return nil, err
	}
	next, err := PopularPostsCursor.Encode(page.Next)
	if err != nil {
		return nil, err
	}
	return &dto.PostsPageResponse{Items: page.Items, NextCursor: next}, nil
}

// ViewPost возвращает пост целиком
// @Summary Просмотр поста
// @Description Возвращает пост целиком. Авторизация необязательна: автор видит свои неопубликованные посты, для остальных они недоступны (404). У закрытого поста (locked = true) поле body пустое.
// @Tags posts
// @Produce json
// @Param id path string true "ID поста" Format(uuid)
// @Success 200 {object} dto.PostInfo "Пост"
// @Failure 400 {object} response.ErrorResponse "Некорректный ID поста"
// @Failure 404 {object} response.ErrorResponse "Пост не найден или недоступен"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /posts/{id} [get]
func (h *PostHandler) ViewPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, err := getPostID(r)
	if err != nil {
		h.handleError(ctx, w, "view post", "post ID invalid", fmt.Errorf("invalid post id: %w: %w", validator.ErrValidation, err))
		return
	}

	userID := h.getUserIDFromContext(ctx)
	post, err := h.uc.GetPost(ctx, userID, postID)
	if err != nil {
		h.handleError(ctx, w, "view post", "get post error", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, post)
}

// NewPost создаёт пост
// @Summary Создание поста
// @Description Создаёт пост от имени текущего пользователя. Создавать посты могут только авторы. Поле last_updated_at при создании не используется. Для статуса published дата публикации проставляется автоматически.
// @Tags posts
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param request body dto.PostRequest true "Данные поста"
// @Success 200 {object} dto.CreatePostResponse "ID созданного поста"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь не является автором, либо неверный CSRF-токен или его отсутствие"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /posts [post]
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

	userID := h.getUserIDFromContext(ctx)
	postID, err := h.uc.CreatePost(ctx, userID, postReq)
	if err != nil {
		h.handleError(ctx, w, "new post", "create post error", err)
		return
	}
	h.logger.InfoContext(ctx, "new post created",
		slog.String("post_id", postID.String()),
		slog.String("userID", userID.String()),
	)
	response.WriteJSON(w, http.StatusOK, dto.CreatePostResponse{ID: postID})
}

// EditPost изменяет пост
// @Summary Изменение поста
// @Description Полностью заменяет title, body и status поста. Используется оптимистическая блокировка: в last_updated_at нужно передать значение updated_at, полученное при последнем чтении поста. Если пост успели изменить, вернётся 409, и пост нужно перечитать.
// @Tags posts
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param id path string true "ID поста" Format(uuid)
// @Param request body dto.PostRequest true "Новые данные поста (last_updated_at обязателен)"
// @Success 200 "Пост обновлён"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации или не передан last_updated_at"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пост принадлежит другому пользователю, либо неверный CSRF-токен или его отсутствие"
// @Failure 404 {object} response.ErrorResponse "Пост не найден или недоступен"
// @Failure 409 {object} response.ErrorResponse "Пост уже был изменён (устаревший last_updated_at)"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /posts/{id} [put]
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

	userID := h.getUserIDFromContext(ctx)
	if err := h.uc.UpdatePost(ctx, userID, postID, postReq, *postReq.LastUpdatedAt); err != nil {
		h.handleError(ctx, w, "edit post", "update post error", err)
		return
	}
	h.logger.InfoContext(ctx, "post edited",
		slog.String("post_id", postID.String()),
		slog.String("userID", userID.String()),
	)
	w.WriteHeader(http.StatusOK)
}

// DeletePost удаляет пост
// @Summary Удаление поста
// @Description Удаляет пост текущего пользователя вместе с лайками, комментариями и уведомлениями.
// @Tags posts
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param id path string true "ID поста" Format(uuid)
// @Success 200 "Пост удалён"
// @Failure 400 {object} response.ErrorResponse "Некорректный ID поста"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Failure 404 {object} response.ErrorResponse "Пост не найден или принадлежит другому пользователю"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /posts/{id} [delete]
func (h *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, err := getPostID(r)
	if err != nil {
		h.handleError(ctx, w, "delete post", "post ID invalid", fmt.Errorf("invalid post id: %w: %w", validator.ErrValidation, err))
		return
	}

	userID := h.getUserIDFromContext(ctx)
	if err := h.uc.DeletePost(ctx, userID, postID); err != nil {
		h.handleError(ctx, w, "delete post", "delete post error", err)
		return
	}
	h.logger.InfoContext(ctx, "post deleted",
		slog.String("post_id", postID.String()),
		slog.String("userID", userID.String()),
	)
	w.WriteHeader(http.StatusOK)
}

// LikePost ставит лайк посту
// @Summary Лайк поста
// @Description Ставит лайк опубликованному посту. Операция идемпотентна: повторный лайк не увеличивает счётчик. Лайкнуть можно только пост, доступный пользователю; автору о лайке своего поста уведомление не отправляется.
// @Tags posts
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param id path string true "ID поста" Format(uuid)
// @Success 200 {object} dto.PostLikeResponse "Актуальное число лайков"
// @Failure 400 {object} response.ErrorResponse "Некорректный ID поста"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Failure 404 {object} response.ErrorResponse "Пост не найден или недоступен"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /posts/{id}/like [post]
func (h *PostHandler) LikePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, err := getPostID(r)
	if err != nil {
		h.handleError(ctx, w, "like post", "post ID invalid", fmt.Errorf("invalid post id: %w: %w", validator.ErrValidation, err))
		return
	}

	userID := h.getUserIDFromContext(ctx)
	likesCount, err := h.uc.LikePost(ctx, userID, postID)
	if err != nil {
		h.handleError(ctx, w, "like post", "like post error", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, dto.PostLikeResponse{LikesCount: likesCount, LikedByUser: true})
}

// UnlikePost снимает лайк с поста
// @Summary Снятие лайка с поста
// @Description Снимает лайк пользователя с поста. Операция идемпотентна: если лайка нет, счётчик не меняется.
// @Tags posts
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param id path string true "ID поста" Format(uuid)
// @Success 200 {object} dto.PostLikeResponse "Актуальное число лайков"
// @Failure 400 {object} response.ErrorResponse "Некорректный ID поста"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Failure 404 {object} response.ErrorResponse "Лайк не найден или пост недоступен"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /posts/{id}/like [delete]
func (h *PostHandler) UnlikePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, err := getPostID(r)
	if err != nil {
		h.handleError(ctx, w, "unlike post", "post ID invalid", fmt.Errorf("invalid post id: %w: %w", validator.ErrValidation, err))
		return
	}

	userID := h.getUserIDFromContext(ctx)
	likesCount, err := h.uc.DeletePostLike(ctx, userID, postID)
	if err != nil {
		h.handleError(ctx, w, "unlike post", "like post error", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, dto.PostLikeResponse{LikesCount: likesCount, LikedByUser: false})
}

func (h *PostHandler) getUserIDFromContext(ctx context.Context) *uuid.UUID {
	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok {
		h.logger.WarnContext(ctx, "posts: invalid user id from context")
	}
	if user == nil {
		return nil
	}
	id := user.UserID
	return &id
}

func getPostID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

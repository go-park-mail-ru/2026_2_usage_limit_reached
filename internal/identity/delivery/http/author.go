package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
)

const (
	authorQueryField    = "q"
	authorCategoryField = "category"
	authorCursorField   = "cursor"
)

type AuthorUsecase interface {
	SearchAuthors(ctx context.Context, req dto.AuthorSearchRequest) (*dto.AuthorSearchPage, error)
	CreateAuthor(ctx context.Context, userID uuid.UUID, req dto.AuthorCreateRequest) (*dto.AuthorMeResponse, error)
	GetAuthor(ctx context.Context, userID uuid.UUID) (*dto.AuthorMeResponse, error)
	UpdateAuthor(ctx context.Context, userID uuid.UUID, req dto.AuthorCreateRequest) (*dto.AuthorMeResponse, error)
}

type AuthorHandler struct {
	authors   AuthorUsecase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewAuthorHandler(authors AuthorUsecase, v *validator.Validator, logger *slog.Logger) *AuthorHandler {
	return &AuthorHandler{authors: authors, validator: v, logger: logger}
}

// SearchAuthors ищет авторов по username, nickname и категориям
// @Summary Поиск авторов
// @Description Ищет активных авторов по частичному совпадению с username или nickname и по категориям. Размер страницы фиксирован, следующая страница запрашивается через next_cursor
// @Tags authors
// @Produce json
// @Param q query string false "Часть username или nickname, минимум 2 символа"
// @Param category query []string false "Категории автора, параметр можно передать несколько раз"
// @Param cursor query string false "Курсор из next_cursor предыдущего ответа"
// @Success 200 {object} dto.AuthorSearchResponse "Страница авторов"
// @Failure 400 {object} response.ErrorResponse "Неверный запрос или курсор"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /authors [get]
func (h *AuthorHandler) SearchAuthors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := r.URL.Query()

	req := dto.AuthorSearchRequest{
		Query:      strings.TrimSpace(params.Get(authorQueryField)),
		Categories: normalizeCategories(params[authorCategoryField]),
	}
	if err := req.Validate(h.validator); err != nil {
		handleError(ctx, h.logger, w, "search authors", fmt.Errorf("invalid request: %w", err))
		return
	}

	cur, err := AuthorSearchCursor.Decode(params.Get(authorCursorField))
	if err != nil {
		handleError(ctx, h.logger, w, "search authors", fmt.Errorf("decode cursor: %w: %w", validator.ErrValidation, err))
		return
	}
	req.Cursor = cur

	page, err := h.authors.SearchAuthors(ctx, req)
	if err != nil {
		handleError(ctx, h.logger, w, "search authors", err)
		return
	}

	next, err := AuthorSearchCursor.Encode(page.Next)
	if err != nil {
		handleError(ctx, h.logger, w, "search authors", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, dto.AuthorSearchResponse{Items: page.Items, NextCursor: next})
}

func normalizeCategories(categories []string) []string {
	var normalized []string
	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category != "" {
			normalized = append(normalized, category)
		}
	}

	return normalized
}

// BecomeAuthor создаёт профиль автора для текущего пользователя
// @Summary Стать автором
// @Description Создаёт профиль автора. Повторный вызов ничего не меняет и возвращает существующий профиль
// @Tags authors
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param request body dto.AuthorCreateRequest true "Данные автора: bio обязателен, category опциональна (Творчество, Образование, Технологии, Игры, Другое)"
// @Success 200 {object} dto.AuthorMeResponse "Профиль автора"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь заблокирован или удалён"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /authors/me [post]
func (h *AuthorHandler) BecomeAuthor(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer r.Body.Close()

	var req dto.AuthorCreateRequest
	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(ctx, h.logger, w, "become author", fmt.Errorf("invalid request body: %w: %w", validator.ErrValidation, err))
		return
	}
	if err := req.Validate(h.validator); err != nil {
		handleError(ctx, h.logger, w, "become author", err)
		return
	}

	userID, ok := userIDFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	author, err := h.authors.CreateAuthor(ctx, userID, req)
	if err != nil {
		handleError(ctx, h.logger, w, "become author", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, author)
}

// GetMeAuthor возвращает профиль автора текущего пользователя
// @Summary Мой профиль автора
// @Description Возвращает блок автора для страницы профиля. 404 означает, что пользователь ещё не автор
// @Tags authors
// @Produce json
// @Security CookieAuth
// @Success 200 {object} dto.AuthorMeResponse "Профиль автора"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь заблокирован или удалён"
// @Failure 404 {object} response.ErrorResponse "Пользователь не автор"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /authors/me [get]
func (h *AuthorHandler) GetMeAuthor(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := userIDFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	author, err := h.authors.GetAuthor(ctx, userID)
	if err != nil {
		handleError(ctx, h.logger, w, "get my author", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, author)
}

// EditAuthor изменяет bio и категорию автора
// @Summary Редактирование профиля автора
// @Description Полностью заменяет bio и category текущего автора; отсутствие category в теле очищает её. Пользователь должен быть автором
// @Tags authors
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param request body dto.AuthorCreateRequest true "Новые данные автора"
// @Success 200 {object} dto.AuthorMeResponse "Обновлённый профиль автора"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь заблокирован или удалён"
// @Failure 404 {object} response.ErrorResponse "Пользователь не автор"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /authors/me [put]
func (h *AuthorHandler) EditAuthor(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer r.Body.Close()

	var req dto.AuthorCreateRequest
	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(ctx, h.logger, w, "edit author", fmt.Errorf("invalid request body: %w: %w", validator.ErrValidation, err))
		return
	}
	if err := req.Validate(h.validator); err != nil {
		handleError(ctx, h.logger, w, "edit author", err)
		return
	}

	userID, ok := userIDFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	author, err := h.authors.UpdateAuthor(ctx, userID, req)
	if err != nil {
		handleError(ctx, h.logger, w, "edit author", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, author)
}

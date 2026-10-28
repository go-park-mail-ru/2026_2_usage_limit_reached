package profilehandlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ProfileUsecase interface {
	GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error)
	GetMyPosts(ctx context.Context, userID uuid.UUID) (*dto.ProfilePostsResponse, error)
	UpdateMyProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) error
}

type ProfileHandler struct {
	uc        ProfileUsecase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewProfileHandler(uc ProfileUsecase, validator *validator.Validator, logger *slog.Logger) *ProfileHandler {
	return &ProfileHandler{uc: uc, validator: validator, logger: logger}
}

func (h *ProfileHandler) RegisterRoutes(public *mux.Router, private *mux.Router) {
	private.HandleFunc("/profile/me", h.GetMe).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me", h.UpdateMe).Methods(http.MethodPatch, http.MethodOptions)
	private.HandleFunc("/profile/me/posts", h.GetMePosts).Methods(http.MethodGet, http.MethodOptions)
}

// GetMe возвращает профиль текущего пользователя
// @Summary Получение профиля текущего пользователя
// @Description Возвращает данные пользователя и информацию о том, является ли он автором
// @Tags profile
// @Produce json
// @Security CookieAuth
// @Success 200 {object} dto.ProfileResponse "Профиль пользователя"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me [get]
func (h *ProfileHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok || id == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	profile, err := h.uc.GetMyProfile(r.Context(), id)
	if err != nil {
		h.handleError(r.Context(), w, "get my profile", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, profile)
}

// GetMePosts возвращает посты текущего пользователя
// @Summary Получение постов текущего пользователя
// @Description Возвращает посты текущего пользователя; если постов нет, возвращает пустой список
// @Tags profile
// @Produce json
// @Security CookieAuth
// @Success 200 {object} dto.ProfilePostsResponse "Посты пользователя"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/posts [get]
func (h *ProfileHandler) GetMePosts(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok || id == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	posts, err := h.uc.GetMyPosts(r.Context(), id)
	if err != nil {
		h.handleError(r.Context(), w, "get my posts", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, posts)
}

// UpdateMe изменяет профиль текущего пользователя
// @Summary Изменение профиля текущего пользователя
// @Description Валидирует и сохраняет данные пользователя и, если передан блок author, данные автора в одной транзакции
// @Tags profile
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param request body dto.UpdateProfileRequest true "Новые данные профиля"
// @Success 200 "Профиль изменен"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь не является автором"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 409 {object} response.ErrorResponse "Username, email или счет выплат уже заняты"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me [patch]
func (h *ProfileHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok || id == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.UpdateProfileRequest
	if err := response.DecodeJSON(w, r, &req); err != nil {
		h.handleError(r.Context(), w, "update my profile", validator.ErrValidation)
		return
	}

	if err := req.Validate(h.validator); err != nil {
		h.handleError(r.Context(), w, "update my profile", err)
		return
	}

	if err := h.uc.UpdateMyProfile(r.Context(), id, req); err != nil {
		h.handleError(r.Context(), w, "update my profile", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ProfileHandler) handleError(ctx context.Context, w http.ResponseWriter, handler string, err error) {
	h.logger.ErrorContext(ctx, "profile error",
		slog.String("handler", handler),
		slog.String("error", err.Error()),
	)

	switch {
	case errors.Is(err, profileusecase.ErrProfileNotFound):
		response.ErrorNotFound(w)
	case errors.Is(err, validator.ErrValidation), errors.Is(err, profileusecase.ErrInvalidProfile):
		response.ErrorBadRequest(w)
	case errors.Is(err, profileusecase.ErrProfileConflict):
		response.ErrorProfileConflict(w)
	case errors.Is(err, profileusecase.ErrNotAuthor):
		response.ErrorForbidden(w)
	default:
		response.ErrorInternal(w)
	}
}

package profilehandlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ProfileUsecase interface {
	GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error)
	GetPublicProfile(ctx context.Context, username string) (*dto.PublicProfileResponse, error)
	UpdateMyProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) (*dto.ProfileResponse, error)
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
	private.HandleFunc("/profile/me", h.UpdateMe).Methods(http.MethodPut, http.MethodOptions)
	public.HandleFunc("/users/{username}", h.GetByUsername).Methods(http.MethodGet, http.MethodOptions)
}

// GetMe возвращает профиль текущего пользователя
// @Summary Получение профиля текущего пользователя
// @Description Возвращает данные пользователя. Блок author есть только у автора, avatar_url - только при наличии аватара
// @Tags profile
// @Produce json
// @Security CookieAuth
// @Success 200 {object} dto.ProfileResponse "Профиль пользователя"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь заблокирован или удалён"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me [get]
func (h *ProfileHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserInfoFromContext(r.Context())
	if !ok || user == nil || user.UserID == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	profile, err := h.uc.GetMyProfile(r.Context(), user.UserID)
	if err != nil {
		h.handleError(r.Context(), w, "get my profile", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, profile)
}

// GetByUsername возвращает публичный профиль пользователя
// @Summary Получение публичного профиля пользователя
// @Description Возвращает публичные данные пользователя без email. Блок author есть только у автора, avatar_url - только при наличии аватара
// @Tags profile
// @Produce json
// @Param username path string true "Username пользователя"
// @Success 200 {object} dto.PublicProfileResponse "Публичный профиль пользователя"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден, заблокирован или удалён"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /users/{username} [get]
func (h *ProfileHandler) GetByUsername(w http.ResponseWriter, r *http.Request) {
	profile, err := h.uc.GetPublicProfile(r.Context(), mux.Vars(r)["username"])
	if err != nil {
		h.handleError(r.Context(), w, "get public profile", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

// UpdateMe изменяет профиль текущего пользователя
// @Summary Изменение профиля текущего пользователя
// @Description Сохраняет nickname и, если передан блок author, bio и category. Если category нет или она null, категория очищается
// @Tags profile
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param request body dto.UpdateProfileRequest true "Новые данные профиля"
// @Success 200 {object} dto.ProfileResponse "Профиль изменен"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь не является автором, заблокирован или удалён"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me [put]
func (h *ProfileHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserInfoFromContext(r.Context())
	if !ok || user == nil || user.UserID == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.UpdateProfileRequest
	if err := response.DecodeJSON(w, r, &req); err != nil {
		h.handleError(r.Context(), w, "update my profile", fmt.Errorf("%w: %v", validator.ErrValidation, err))
		return
	}

	if err := req.Validate(h.validator); err != nil {
		h.handleError(r.Context(), w, "update my profile", err)
		return
	}

	profile, err := h.uc.UpdateMyProfile(r.Context(), user.UserID, req)
	if err != nil {
		h.handleError(r.Context(), w, "update my profile", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

func (h *ProfileHandler) handleError(ctx context.Context, w http.ResponseWriter, handler string, err error) {
	level := slog.LevelInfo

	switch {
	case errors.Is(err, profileusecase.ErrProfileNotFound):
		response.ErrorNotFound(w)
	case errors.Is(err, validator.ErrValidation):
		response.ErrorBadRequest(w)
	case errors.Is(err, profileusecase.ErrNotAuthor), errors.Is(err, profileusecase.ErrUserInactive):
		response.ErrorForbidden(w)
	default:
		level = slog.LevelError

		response.ErrorInternal(w)
	}

	h.logger.Log(ctx, level, "profile error",
		slog.String("handler", handler),
		slog.String("error", err.Error()),
	)
}

package profilehandlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/domain"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ProfileUsecase interface {
	GetMyProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error)
}

type ProfileHandler struct {
	uc     ProfileUsecase
	logger *slog.Logger
}

func NewProfileHandler(uc ProfileUsecase, logger *slog.Logger) *ProfileHandler {
	return &ProfileHandler{uc: uc, logger: logger}
}

func (h *ProfileHandler) RegisterRoutes(public *mux.Router, private *mux.Router) {
	private.HandleFunc("/profile/me", h.GetMe).Methods(http.MethodGet, http.MethodOptions)
}

// GetMe возвращает профиль текущего пользователя
// @Summary Получение профиля текущего пользователя
// @Description Возвращает данные пользователя, инфо об авторе и его посты
// @Tags profile
// @Produce json
// @Security CookieAuth
// @Success 200 {object} dto.ProfileResponse "Профиль пользователя"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
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
	response.WriteJSON(w, http.StatusOK, dto.ToProfileResponse(*profile))
}

func (h *ProfileHandler) handleError(ctx context.Context, w http.ResponseWriter, handler string, err error) {
	h.logger.ErrorContext(ctx, "profile error",
		slog.String("handler", handler),
		slog.String("error", err.Error()),
	)

	switch {
	case errors.Is(err, profileusecase.ErrProfileNotFound):
		response.ErrorUnauthorized(w)
	default:
		response.ErrorInternal(w)
	}
}

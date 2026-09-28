package profilehandlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/delivery/handlers/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/domain"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ProfileUsecase interface {
	GetMyProfile(ctx context.Context, userID uuid.UUID) (domain.Profile, error)
}

type ProfileHandler struct {
	uc            ProfileUsecase
	currentUserID func(context.Context) (uuid.UUID, bool)
	logger        *slog.Logger
}

func NewProfileHandler(uc ProfileUsecase, currentUserID func(context.Context) (uuid.UUID, bool), logger *slog.Logger) *ProfileHandler {
	return &ProfileHandler{uc: uc, currentUserID: currentUserID, logger: logger}
}

func (h *ProfileHandler) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Handle("/profile/me", authMiddleware(http.HandlerFunc(h.GetMe))).Methods(http.MethodGet)
}

func (h *ProfileHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	id, ok := h.currentUserID(r.Context())
	if !ok || id == uuid.Nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	
	profile, err := h.uc.GetMyProfile(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			response.Error(w, http.StatusUnauthorized, "unauthorized")
		} else {
			h.logger.Error("get my profile", "error", err)
			response.Error(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	response.WriteJSON(w, http.StatusOK, dto.ToProfileResponse(profile))
}

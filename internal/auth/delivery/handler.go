package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/gorilla/mux"
)

type UseCase interface {
	Register(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error)
	Login(ctx context.Context, loginInput dto.LoginRequest) (*dto.UserResponse, string, error)
	TokenTTL() time.Duration
}

type Handler struct {
	uc        UseCase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewHandler(uc UseCase, validator *validator.Validator, logger *slog.Logger) *Handler {
	return &Handler{uc: uc, validator: validator, logger: logger}
}

func (h *Handler) RegisterRoutes(public *mux.Router, private *mux.Router) {
	public.HandleFunc("/signup", h.Register).Methods(http.MethodPost, http.MethodOptions)
	public.HandleFunc("/login", h.Login).Methods(http.MethodPost, http.MethodOptions)

	private.HandleFunc("/logout", h.Logout).Methods(http.MethodPost, http.MethodOptions)
}

func (h *Handler) handleError(ctx context.Context, w http.ResponseWriter, handler string, logMessage string, err error) {
	h.logger.ErrorContext(ctx, logMessage,
		slog.String("handler", handler),
		slog.String("error", err.Error()),
	)
	switch {
	case errors.Is(err, usecase.ErrRegistrationFailed):
		response.ErrorBadRequest(w)
	case errors.Is(err, usecase.ErrLoginFailed):
		response.ErrorUnauthorized(w)
	default:
		response.ErrorInternal(w)
	}
}

func (h *Handler) setAuthCookie(w http.ResponseWriter, jwtToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    jwtToken,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(h.uc.TokenTTL()),
		MaxAge:   int(h.uc.TokenTTL().Seconds()),
	})
}

func (h *Handler) deleteAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now(),
		MaxAge:   -1,
	})
}

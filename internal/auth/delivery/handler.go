package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

var (
	ErrInternal     = errors.New("internal server error")
	ErrBadRequest   = errors.New("bad request")
	ErrUnauthorized = errors.New("unauthorized")
)

type UseCase interface {
	Register(ctx context.Context, regInput usecase.RegisterInput) (*models.User, string, error)
	Login(ctx context.Context, username, email, password string) (*models.User, string, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

type Handler struct {
	uc        UseCase
	tokenTTL  time.Duration
	validator validator.Validator
	logger    *slog.Logger
}

func NewHandler(uc UseCase, tokenTTL time.Duration, validator validator.Validator, logger *slog.Logger) *Handler {
	return &Handler{uc: uc, tokenTTL: tokenTTL, validator: validator, logger: logger}
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
		response.Error(w, http.StatusConflict, ErrBadRequest.Error())
	case errors.Is(err, usecase.ErrLoginFailed):
		response.Error(w, http.StatusUnauthorized, ErrUnauthorized.Error())
	case errors.Is(err, dto.ErrValidation):
		response.Error(w, http.StatusBadRequest, ErrBadRequest.Error())
	case errors.Is(err, ErrBadRequest):
		response.Error(w, http.StatusBadRequest, ErrBadRequest.Error())
	case errors.Is(err, usecase.ErrInternal):
		response.Error(w, http.StatusInternalServerError, ErrInternal.Error())
	default:
		response.Error(w, http.StatusInternalServerError, ErrInternal.Error())
	}
}

func (h *Handler) setAuthCookie(w http.ResponseWriter, jwtToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    jwtToken,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(h.tokenTTL),
		MaxAge:   int(h.tokenTTL.Seconds()),
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

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery/handlers/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/gorilla/mux"
)

type UseCase interface {
	Register(ctx context.Context, email, username, nickname, password string) (models.User, string, error)
	Login(ctx context.Context, email, password string) (models.User, string, error)
	GetUserFromContext(ctx context.Context) (models.User, error)
}

type Handler struct {
	uc       UseCase
	tokenTTL time.Duration
}

func NewHandler(uc UseCase, tokenTTL time.Duration) *Handler {
	return &Handler{uc: uc, tokenTTL: tokenTTL}
}

func (h *Handler) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	r.HandleFunc("/signup", h.Register).Methods("POST")
	r.HandleFunc("/login", h.Login).Methods("POST")

	r.Handle("/me", authMiddleware(http.HandlerFunc(h.Me))).Methods(http.MethodGet)
	r.Handle("/logout", authMiddleware(http.HandlerFunc(h.Logout))).Methods(http.MethodPost)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var regReq dto.RegistrationRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&regReq); err != nil {
		h.handleError(w, models.ErrValidation)
		return
	}

	user, token, err := h.uc.Register(r.Context(), regReq.Email, regReq.Username, regReq.Nickname, regReq.Password)
	if err != nil {
		h.handleError(w, err)
		return
	}
	h.setAuthCookie(w, token)
	response.WriteJSON(w, http.StatusCreated, toUserResponse(user))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var loginReq dto.LoginRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		h.handleError(w, models.ErrValidation)
		return
	}

	user, token, err := h.uc.Login(r.Context(), loginReq.Email, loginReq.Password)
	if err != nil {
		h.handleError(w, err)
		return
	}
	h.setAuthCookie(w, token)
	response.WriteJSON(w, http.StatusOK, toUserResponse(user))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now(),
		MaxAge:   -1,
	})
	response.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"}) // договориться о контракте
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.uc.GetUserFromContext(r.Context())
	if err != nil {
		h.handleError(w, err)
	}
	response.WriteJSON(w, http.StatusOK, toUserResponse(user))
}

func (h *Handler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, models.ErrUserExists):
		response.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, models.ErrInvalidCredentials):
		response.Error(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, models.ErrUnauthorized):
		response.Error(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, models.ErrValidation):
		response.Error(w, http.StatusBadRequest, err.Error())
	default:
		response.Error(w, http.StatusInternalServerError, "internal error")
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

func toUserResponse(user models.User) dto.UserResponse {
	return dto.UserResponse{
		Email:    user.Email,
		Username: user.Username,
		Nickname: user.Nickname,
	}
}

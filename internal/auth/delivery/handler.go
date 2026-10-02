package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
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
	Login(ctx context.Context, username, password string) (*models.User, string, error)
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
	public.HandleFunc("/signup", h.Register).Methods(http.MethodPost)
	public.HandleFunc("/login", h.Login).Methods(http.MethodPost)

	private.HandleFunc("/logout", h.Logout).Methods(http.MethodPost)
}

// Register регистрирует нового пользователя
// @Summary      Регистрация пользователя
// @Description  Принимает регистрационные данные, валидирует и создает новый аккаунт
// @Tags	     auth
// @Accept       json
// @Produce      json
// @Param        request body dto.RegistrationRequest true "Данные нового пользователя"
// @Success      200  {object}  dto.UserResponse "Пользователь успешно зарегистрирован"
// @Failure      400  {object}  response.ErrorResponse "Ошибка валидации"
// @Failure      409  {object}  response.ErrorResponse "Ошибка регистрации: пользователь с таким email / username уже существует"
// @Failure      500  {object}  response.ErrorResponse "Внутренняя ошибка сервера"
// @Router       /signup [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var regReq dto.RegistrationRequest
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&regReq); err != nil {
		h.handleError(r.Context(), w, "register", "request body decode error", ErrBadRequest)
		return
	}

	if err := regReq.Validate(h.validator); err != nil {
		h.handleError(r.Context(), w, "register", "validation error", err)
		return
	}

	h.logger.InfoContext(r.Context(), "registering user",
		slog.String("email", regReq.Email),
	)

	user, token, err := h.uc.Register(r.Context(), usecase.RegisterInput{
		Email:    regReq.Email,
		Username: regReq.Username,
		Nickname: regReq.Nickname,
		Password: regReq.Password,
	})
	if err != nil {
		h.handleError(r.Context(), w, "register", "register error", err)
		return
	}

	h.logger.InfoContext(r.Context(), "user registered",
		slog.String("userID", user.ID.String()),
	)

	h.setAuthCookie(w, token)

	response.WriteJSON(w, http.StatusOK, dto.ToUserResponse(*user))
}

// Login аутентифицирует пользователя
// @Summary Аутентификация пользователя
// @Description Принимает учетные данные, проверяет их и выпускает токен
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Учетные данные пользователя"
// @Success 200 {object} dto.UserResponse "Успешная аутентификация"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Неверный логин или пароль"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Header 200 {string} Set-Cookie "token=JWT_TOKEN; Path=/; HttpOnly; SameSite=Lax"
// @Router /login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var loginReq dto.LoginRequest
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		h.handleError(r.Context(), w, "login", "request body decode error", ErrBadRequest)
		return
	}

	if err := loginReq.Validate(h.validator); err != nil {
		h.handleError(r.Context(), w, "login", "validation error", err)
		return
	}

	h.logger.InfoContext(r.Context(), "logging in user",
		slog.String("username", loginReq.Username),
	)

	user, token, err := h.uc.Login(r.Context(), loginReq.Username, loginReq.Password)
	if err != nil {
		h.handleError(r.Context(), w, "login", "login error", err)
		return
	}

	h.logger.InfoContext(r.Context(), "user logged in",
		slog.String("userID", user.ID.String()),
	)

	h.setAuthCookie(w, token)
	response.WriteJSON(w, http.StatusOK, dto.ToUserResponse(*user))
}

// Logout деавторизует пользователя
// @Summary Деавторизация пользователя
// @Description Удаляет авторизационную cookie с токеном
// @Tags auth
// @Produce json
// @Security CookieAuth
// @Success 200 {object} map[string]string "Успешный выход из системы"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Header 200 {string} Set-Cookie "token=; Path=/; Max-Age=-1"
// @Router /logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	id, _ := middleware.UserIDFromContext(r.Context())
	h.logger.InfoContext(r.Context(), "logging out user",
		slog.String("userID", id.String()),
	)
	h.deleteAuthCookie(w)
	response.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
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

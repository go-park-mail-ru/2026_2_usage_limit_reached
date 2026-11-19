package handlers

import (
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type AuthHandler struct {
	auth      AuthUsecase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewAuthHandler(auth AuthUsecase, validator *validator.Validator, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, validator: validator, logger: logger}
}

// Register регистрирует нового пользователя
// @Summary Регистрация пользователя
// @Description Принимает регистрационные данные и создает новый аккаунт
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegistrationRequest true "Данные нового пользователя"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 200 {object} dto.UserResponse "Пользователь успешно зарегистрирован"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 403 {object} response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Failure 409 {object} response.ErrorResponse "Пользователь уже существует"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Header 200 {string} Set-Cookie "__Host-token=JWT_TOKEN; Path=/; Secure; HttpOnly; SameSite=Lax"
// @Router /signup [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegistrationRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		logError(r.Context(), h.logger, "register", err)
		response.ErrorBadRequest(w)
		return
	}

	if err := req.Validate(h.validator); err != nil {
		handleError(r.Context(), h.logger, w, "register", err)
		return
	}

	user, token, err := h.auth.Register(r.Context(), req)
	if err != nil {
		handleError(r.Context(), h.logger, w, "register", err)
		return
	}

	setAuthCookie(w, token, h.auth.TokenTTL())
	deleteCSRFCookie(w)
	response.WriteJSON(w, http.StatusOK, user)
}

// Login аутентифицирует пользователя
// @Summary Аутентификация пользователя
// @Description Принимает учетные данные и выпускает JWT
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Учетные данные пользователя"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 200 {object} dto.UserResponse "Успешная аутентификация"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Неверный логин или пароль"
// @Failure 403 {object} response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Header 200 {string} Set-Cookie "__Host-token=JWT_TOKEN; Path=/; Secure; HttpOnly; SameSite=Lax"
// @Router /login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		logError(r.Context(), h.logger, "login", err)
		response.ErrorBadRequest(w)
		return
	}

	if err := req.Validate(); err != nil {
		handleError(r.Context(), h.logger, w, "login", err)
		return
	}

	user, token, err := h.auth.Login(r.Context(), req)
	if err != nil {
		handleError(r.Context(), h.logger, w, "login", err)
		return
	}

	setAuthCookie(w, token, h.auth.TokenTTL())
	deleteCSRFCookie(w)
	response.WriteJSON(w, http.StatusOK, user)
}

// Logout удаляет авторизационную cookie
// @Summary Деавторизация пользователя
// @Tags auth
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 200 "Успешный выход из системы"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Header 200 {string} Set-Cookie "__Host-token=; Path=/; Secure; Max-Age=-1"
// @Router /logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	deleteAuthCookie(w)
	deleteCSRFCookie(w)
	w.WriteHeader(http.StatusOK)
}

package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

// Register регистрирует нового пользователя
// @Summary      Регистрация пользователя
// @Description  Принимает регистрационные данные, валидирует и создает новый аккаунт
// @Tags	     auth
// @Accept       json
// @Produce      json
// @Param        request body dto.RegistrationRequest true "Данные нового пользователя"
// @Param        X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success      200  {object}  dto.UserResponse "Пользователь успешно зарегистрирован"
// @Failure      400  {object}  response.ErrorResponse "Ошибка валидации"
// @Failure      403  {object}  response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Failure      409  {object}  response.ErrorResponse "Ошибка регистрации: пользователь с таким email / username уже существует"
// @Failure      500  {object}  response.ErrorResponse "Внутренняя ошибка сервера"
// @Header       200  {string}  Set-Cookie "__Host-token=JWT_TOKEN; Path=/; Secure; HttpOnly; SameSite=Lax"
// @Router       /signup [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var regReq dto.RegistrationRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &regReq); err != nil {
		h.handleError(r.Context(), w, "register", "request body decode error", fmt.Errorf("%w: %v", validator.ErrValidation, err))
		return
	}

	if err := regReq.Validate(h.validator); err != nil {
		h.handleError(r.Context(), w, "register", "validation error", err)
		return
	}

	user, token, err := h.uc.Register(r.Context(), regReq)
	if err != nil {
		h.handleError(r.Context(), w, "register", "register error", err)
		return
	}

	h.setAuthCookie(w, token)
	h.deleteContextCookie(w)
	response.WriteJSON(w, http.StatusOK, user)
}

// Login аутентифицирует пользователя
// @Summary Аутентификация пользователя
// @Description Принимает учетные данные, проверяет их и выпускает токен
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Учетные данные пользователя"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 200 {object} dto.UserResponse "Успешная аутентификация"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Неверный логин или пароль, либо аккаунт неактивен"
// @Failure 403 {object} response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Header 200 {string} Set-Cookie "__Host-token=JWT_TOKEN; Path=/; Secure; HttpOnly; SameSite=Lax"
// @Router /login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var loginReq dto.LoginRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &loginReq); err != nil {
		h.handleError(r.Context(), w, "login", "request body decode error", fmt.Errorf("%w: %v", validator.ErrValidation, err))
		return
	}

	if err := loginReq.Validate(); err != nil {
		h.handleError(r.Context(), w, "login", "validation error", err)
		return
	}

	user, token, err := h.uc.Login(r.Context(), loginReq)
	if err != nil {
		h.handleError(r.Context(), w, "login", "login error", err)
		return
	}

	h.setAuthCookie(w, token)
	h.deleteContextCookie(w)
	response.WriteJSON(w, http.StatusOK, user)
}

// Logout деавторизует пользователя
// @Summary Деавторизация пользователя
// @Description Удаляет авторизационную cookie с токеном
// @Tags auth
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 200 "Успешный выход из системы"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Header 200 {string} Set-Cookie "__Host-token=; Path=/; Secure; Max-Age=-1"
// @Router /logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.deleteAuthCookie(w)
	h.deleteContextCookie(w)
	w.WriteHeader(http.StatusOK)
}

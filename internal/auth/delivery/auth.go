package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

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
		h.handleError(r.Context(), w, "register", "request body decode error", errBadRequest)
		return
	}

	if err := regReq.Validate(h.validator); err != nil {
		h.handleError(r.Context(), w, "register", "validation error", errBadRequest)
		return
	}

	user, token, err := h.uc.Register(r.Context(), regReq)
	if err != nil {
		h.handleError(r.Context(), w, "register", "register error", err)
		return
	}

	h.logger.InfoContext(r.Context(), "user registered",
		slog.String("userID", user.Email),
	)

	h.setAuthCookie(w, token)

	response.WriteJSON(w, http.StatusOK, user)
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
		h.handleError(r.Context(), w, "login", "request body decode error", errBadRequest)
		return
	}

	if err := loginReq.Validate(h.validator); err != nil {
		h.handleError(r.Context(), w, "login", "validation error", errBadRequest)
		return
	}

	user, token, err := h.uc.Login(r.Context(), loginReq)
	if err != nil {
		h.handleError(r.Context(), w, "login", "login error", err)
		return
	}

	h.logger.InfoContext(r.Context(), "user logged in",
		slog.String("userID", user.Email),
	)

	h.setAuthCookie(w, token)
	response.WriteJSON(w, http.StatusOK, user)
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
	w.WriteHeader(http.StatusOK)
}

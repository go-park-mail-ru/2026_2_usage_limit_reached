package handlers

import (
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type CSRFHandler struct {
	csrf   *csrf.Manager
	logger *slog.Logger
}

func NewCSRFHandler(csrfManager *csrf.Manager, logger *slog.Logger) *CSRFHandler {
	return &CSRFHandler{csrf: csrfManager, logger: logger}
}

// GetToken выдает CSRF-токен
// @Summary Получение CSRF-токена
// @Description Возвращает токен для заголовка X-CSRF-Token. Доступен до и после входа
// @Tags auth
// @Produce json
// @Success 200 {object} dto.CSRFTokenResponse "Токен выдан"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Header 200 {string} Set-Cookie "__Host-csrf=CSRF_CONTEXT; Path=/; Secure; HttpOnly; SameSite=Lax"
// @Router /csrf [get]
func (h *CSRFHandler) GetToken(w http.ResponseWriter, r *http.Request) {
	binding, newAnonContext, err := h.csrf.Resolve(middleware.CSRFInput(r))
	if err != nil {
		if h.logger != nil {
			h.logger.ErrorContext(r.Context(), "issue csrf token error", slog.String("handler", "csrf token"), slog.String("error", err.Error()))
		}
		response.ErrorInternal(w)
		return
	}

	if newAnonContext != "" {
		setCSRFCookie(w, newAnonContext, h.csrf.ContextTTL())
	}

	token, err := h.csrf.Token(binding)
	if err != nil {
		if h.logger != nil {
			h.logger.ErrorContext(r.Context(), "issue csrf token error", slog.String("handler", "csrf token"), slog.String("error", err.Error()))
		}
		response.ErrorInternal(w)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	response.WriteJSON(w, http.StatusOK, dto.CSRFTokenResponse{CSRFToken: token})
}

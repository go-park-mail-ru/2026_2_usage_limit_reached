package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type CSRFTokenResponse struct {
	CSRFToken string `json:"csrf_token" example:"q3VtX2Jhc2U2NHVybF9zYWx0X2FuZF9obWFjX3NpZ25hdHVyZV9leGFtcGxlXzAx"`
}

// CSRFHandler выдаёт токен для мутаций админ-API — тот же механизм,
// что в остальных сервисах (админ-SPA живёт на домене продукта).
type CSRFHandler struct {
	csrf   *csrf.Manager
	logger *slog.Logger
}

func NewCSRFHandler(csrfManager *csrf.Manager, logger *slog.Logger) *CSRFHandler {
	return &CSRFHandler{csrf: csrfManager, logger: logger}
}

// GetToken выдает CSRF-токен
// @Summary Получение CSRF-токена OnlineConf
// @Description Возвращает токен для заголовка X-CSRF-Token мутаций админ-API
// @Tags onlineconf-access
// @Produce json
// @Success 200 {object} handlers.CSRFTokenResponse "Токен выдан"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Header 200 {string} Set-Cookie "__Host-csrf=CSRF_CONTEXT; Path=/; Secure; HttpOnly; SameSite=Lax"
// @Router /csrf [get]
func (h *CSRFHandler) GetToken(w http.ResponseWriter, r *http.Request) {
	binding, newAnonContext, err := h.csrf.Resolve(middleware.CSRFInput(r))
	if err != nil {
		h.logError(r.Context(), err)
		response.ErrorInternal(w)
		return
	}

	if newAnonContext != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     csrf.ContextCookieName,
			Value:    newAnonContext,
			HttpOnly: true,
			Secure:   true,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(h.csrf.ContextTTL()),
			MaxAge:   int(h.csrf.ContextTTL().Seconds()),
		})
	}

	token, err := h.csrf.Token(binding)
	if err != nil {
		h.logError(r.Context(), err)
		response.ErrorInternal(w)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	response.WriteJSON(w, http.StatusOK, CSRFTokenResponse{CSRFToken: token})
}

func (h *CSRFHandler) logError(ctx context.Context, err error) {
	if h.logger == nil {
		return
	}
	h.logger.ErrorContext(ctx, "issue csrf token error", slog.String("error", err.Error()))
}

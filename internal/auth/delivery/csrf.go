package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/gorilla/mux"
)

type CSRFManager interface {
	Resolve(in csrf.BindingInput) (string, string, error)
	Token(binding string) (string, error)
	ContextTTL() time.Duration
}

type CSRFHandler struct {
	csrf   CSRFManager
	logger *slog.Logger
}

func NewCSRFHandler(csrf CSRFManager, logger *slog.Logger) *CSRFHandler {
	return &CSRFHandler{csrf: csrf, logger: logger}
}

func (h *CSRFHandler) RegisterRoutes(public *mux.Router) {
	public.HandleFunc("/csrf", h.GetToken).Methods(http.MethodGet, http.MethodOptions)
}

// GetToken выдает CSRF-токен
// @Summary Получение CSRF-токена
// @Description Возвращает токен для заголовка X-CSRF-Token. Доступен до и после входа, авторизацию не меняет. Токен авторизованного пользователя привязан к его JWT, после входа или выхода нужно получить новый. Неавторизованному пользователю создает cookie с анонимным CSRF-контекстом, если ее нет
// @Tags auth
// @Produce json
// @Success 200 {object} dto.CSRFTokenResponse "Токен выдан"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Header 200 {string} Set-Cookie "__Host-csrf=<context>; Path=/; Secure; HttpOnly; SameSite=Lax"
// @Router /csrf [get]
func (h *CSRFHandler) GetToken(w http.ResponseWriter, r *http.Request) {
	token, err := h.issueToken(w, r)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "issue csrf token error", slog.String("handler", "csrf token"), slog.String("error", err.Error()))
		response.ErrorInternal(w)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	response.WriteJSON(w, http.StatusOK, dto.CSRFTokenResponse{CSRFToken: token})
}

func (h *CSRFHandler) issueToken(w http.ResponseWriter, r *http.Request) (string, error) {
	binding, err := h.currentBinding(w, r)
	if err != nil {
		return "", err
	}

	return h.csrf.Token(binding)
}

func (h *CSRFHandler) currentBinding(w http.ResponseWriter, r *http.Request) (string, error) {
	binding, newAnonContext, err := h.csrf.Resolve(middleware.CSRFInput(r))
	if err != nil {
		return "", err
	}

	if newAnonContext != "" {
		h.setContextCookie(w, newAnonContext)
	}

	return binding, nil
}

func (h *CSRFHandler) setContextCookie(w http.ResponseWriter, csrfContext string) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrf.ContextCookieName,
		Value:    csrfContext,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(h.csrf.ContextTTL()),
		MaxAge:   int(h.csrf.ContextTTL().Seconds()),
	})
}

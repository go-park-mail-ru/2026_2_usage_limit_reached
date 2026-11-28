package handlers

import (
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type AccessHandler struct {
	logger *slog.Logger
}

func NewAccessHandler(logger *slog.Logger) *AccessHandler {
	return &AccessHandler{logger: logger}
}

// GetAccess проверяет право управления OnlineConf
// @Summary Проверка доступа к админке OnlineConf
// @Description Первый запрос админ-SPA: 200 — панель, 401 — редирект на логин, 403 — экран «нет доступа»
// @Tags onlineconf-access
// @Produce json
// @Success 200 {object} dto.AccessResponse "Право есть"
// @Failure 401 {object} response.ErrorResponse "Сессии нет или она недействительна"
// @Failure 403 {object} response.ErrorResponse "Нет права управлять OnlineConf"
// @Router /onlineconf/admin/access [get]
func (h *AccessHandler) GetAccess(w http.ResponseWriter, r *http.Request) {
	response.WriteJSON(w, http.StatusOK, dto.AccessResponse{Allowed: true})
}

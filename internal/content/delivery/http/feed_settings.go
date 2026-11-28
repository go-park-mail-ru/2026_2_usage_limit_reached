package handlers

import (
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

// FeedSettings отдаёт состояние расширенных фильтров ленты
// @Summary Настройки ленты
// @Description Итоговое состояние флага content/feed_attachment_filter для текущего пользователя (с учётом раскатки и персональных правил). Доступен гостю — у него фильтры выключены.
// @Tags posts
// @Produce json
// @Success 200 {object} dto.FeedSettings "Настройки ленты"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /feed/settings [get]
func (h *PostHandler) FeedSettings(w http.ResponseWriter, r *http.Request) {
	settings := h.uc.FeedSettings(r.Context(), h.getUserIDFromContext(r.Context()))

	// Ответ персональный (зависит от пользователя) и динамический:
	// кэшировать его нельзя ни в браузере, ни в промежуточных прокси.
	w.Header().Set("Cache-Control", "private, no-store")
	response.WriteJSON(w, http.StatusOK, settings)
}

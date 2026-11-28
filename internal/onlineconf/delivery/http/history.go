package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

const (
	defaultHistoryLimit = 20
	maxHistoryLimit     = 100
)

type HistoryHandler struct {
	history AdminUsecase
	logger  *slog.Logger
}

func NewHistoryHandler(history AdminUsecase, logger *slog.Logger) *HistoryHandler {
	return &HistoryHandler{history: history, logger: logger}
}

// ListHistory отдаёт журнал изменений с курсорной пагинацией
// @Summary История изменений OnlineConf (бонус)
// @Description Кто, когда и что изменил; курсорная пагинация pkg/cursor, порядок id DESC
// @Tags onlineconf-admin
// @Produce json
// @Param entity_type query string false "Фильтр: config | flag (обязателен вместе со scope/key)"
// @Param scope query string false "Фильтр по скоупу"
// @Param key query string false "Фильтр по ключу"
// @Param limit query int false "Размер страницы (1..100, по умолчанию 20)"
// @Param cursor query string false "Курсор предыдущей страницы"
// @Success 200 {object} dto.HistoryPage "Страница истории"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/history [get]
func (h *HistoryHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := dto.HistoryFilter{
		EntityType: query.Get("entity_type"),
		Scope:      query.Get("scope"),
		Key:        query.Get("key"),
	}

	if filter.Scope != "" || filter.Key != "" {
		if filter.EntityType != "config" && filter.EntityType != "flag" {
			handleError(r.Context(), h.logger, w, "list history", validator.ErrValidation)
			return
		}
	}

	limit := defaultHistoryLimit
	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxHistoryLimit {
			handleError(r.Context(), h.logger, w, "list history", validator.ErrValidation)
			return
		}
		limit = parsed
	}

	var cursorID *int64
	if raw := query.Get("cursor"); raw != "" {
		parsed, err := HistoryCursor.Decode(raw)
		if err != nil {
			handleError(r.Context(), h.logger, w, "list history", err)
			return
		}
		cursorID = &parsed.ID
	}

	items, nextCursor, err := h.history.ListHistory(r.Context(), filter, cursorID, limit)
	if err != nil {
		handleError(r.Context(), h.logger, w, "list history", err)
		return
	}

	page := dto.HistoryPage{Items: items}
	if nextCursor != nil {
		encoded, err := HistoryCursor.Encode(&dto.HistoryCursor{ID: *nextCursor})
		if err != nil {
			handleError(r.Context(), h.logger, w, "list history", err)
			return
		}
		page.Cursor = encoded
	}

	response.WriteJSON(w, http.StatusOK, page)
}

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type ScopesHandler struct {
	scopes AdminUsecase
	logger *slog.Logger
}

func NewScopesHandler(scopes AdminUsecase, logger *slog.Logger) *ScopesHandler {
	return &ScopesHandler{scopes: scopes, logger: logger}
}

// ListScopes отдаёт список скоупов
// @Summary Список скоупов OnlineConf
// @Tags onlineconf-admin
// @Produce json
// @Success 200 {array} dto.ScopeResponse "Скоупы"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/scopes [get]
func (h *ScopesHandler) ListScopes(w http.ResponseWriter, r *http.Request) {
	scopes, err := h.scopes.ListScopes(r.Context())
	if err != nil {
		handleError(r.Context(), h.logger, w, "list scopes", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, scopes)
}

// CreateScope создаёт скоуп
// @Summary Создание скоупа OnlineConf
// @Tags onlineconf-admin
// @Accept json
// @Produce json
// @Param request body dto.ScopeRequest true "Новый скоуп"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 201 {object} dto.ScopeResponse "Скоуп создан"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 409 {object} response.ErrorResponse "Скоуп уже существует"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/scopes [post]
func (h *ScopesHandler) CreateScope(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.ScopeRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(r.Context(), h.logger, w, "create scope", err)
		return
	}

	if err := req.Validate(); err != nil {
		handleError(r.Context(), h.logger, w, "create scope", err)
		return
	}

	scope, err := h.scopes.CreateScope(r.Context(), actor, req.Name, req.Description)
	if err != nil {
		handleError(r.Context(), h.logger, w, "create scope", err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, scope)
}

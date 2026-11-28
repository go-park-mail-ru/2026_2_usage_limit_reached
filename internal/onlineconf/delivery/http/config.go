package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type ConfigsHandler struct {
	configs AdminUsecase
	logger  *slog.Logger
}

func NewConfigsHandler(configs AdminUsecase, logger *slog.Logger) *ConfigsHandler {
	return &ConfigsHandler{configs: configs, logger: logger}
}

// ListConfigs отдаёт конфиги скоупа с поиском по названию
// @Summary Список конфигов OnlineConf
// @Description Поиск по названию (ILIKE) и фильтр по скоупу — обязательное требование задания
// @Tags onlineconf-admin
// @Produce json
// @Param scope query string false "Фильтр по скоупу"
// @Param q query string false "Поиск по названию"
// @Success 200 {array} dto.ConfigResponse "Конфиги"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/configs [get]
func (h *ConfigsHandler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	configs, err := h.configs.ListConfigs(r.Context(), query.Get("scope"), query.Get("q"))
	if err != nil {
		handleError(r.Context(), h.logger, w, "list configs", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, configs)
}

// CreateConfig создаёт конфиг
// @Summary Создание конфига OnlineConf
// @Description value — строка, содержащая JSON: "300" для scalar, сериализованный JSON для json/array
// @Tags onlineconf-admin
// @Accept json
// @Produce json
// @Param request body dto.ConfigRequest true "Новый конфиг"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 201 {object} dto.ConfigResponse "Конфиг создан"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации (в т.ч. невалидный JSON в value)"
// @Failure 404 {object} response.ErrorResponse "Скоуп не найден"
// @Failure 409 {object} response.ErrorResponse "Конфиг уже существует"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/configs [post]
func (h *ConfigsHandler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.ConfigRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(r.Context(), h.logger, w, "create config", err)
		return
	}

	if err := req.Validate(); err != nil {
		handleError(r.Context(), h.logger, w, "create config", err)
		return
	}

	cfg, err := h.configs.CreateConfig(r.Context(), actor, req.Scope, req.Key, req.Value, req.ValueType, req.Description)
	if err != nil {
		handleError(r.Context(), h.logger, w, "create config", err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, cfg)
}

// GetConfig отдаёт конфиг по скоупу и ключу
// @Summary Чтение конфига OnlineConf
// @Tags onlineconf-admin
// @Produce json
// @Param scope path string true "Скоуп"
// @Param key path string true "Ключ"
// @Success 200 {object} dto.ConfigResponse "Конфиг"
// @Failure 404 {object} response.ErrorResponse "Конфиг не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/configs/{scope}/{key} [get]
func (h *ConfigsHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	cfg, err := h.configs.GetConfig(r.Context(), vars["scope"], vars["key"])
	if err != nil {
		handleError(r.Context(), h.logger, w, "get config", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, cfg)
}

// UpdateConfig обновляет значение конфига
// @Summary Обновление конфига OnlineConf
// @Description value — строка, содержащая JSON; смена значения применяется у потребителей не позднее чем через минуту
// @Tags onlineconf-admin
// @Accept json
// @Produce json
// @Param scope path string true "Скоуп"
// @Param key path string true "Ключ"
// @Param request body dto.ConfigRequest true "Новое значение"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 200 {object} dto.ConfigResponse "Конфиг обновлён"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 404 {object} response.ErrorResponse "Конфиг не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/configs/{scope}/{key} [put]
func (h *ConfigsHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.ConfigRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(r.Context(), h.logger, w, "update config", err)
		return
	}

	if err := req.ValidateValue(); err != nil {
		handleError(r.Context(), h.logger, w, "update config", err)
		return
	}

	vars := mux.Vars(r)

	cfg, err := h.configs.UpdateConfig(r.Context(), actor, vars["scope"], vars["key"], req.Value, req.ValueType, req.Description)
	if err != nil {
		handleError(r.Context(), h.logger, w, "update config", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, cfg)
}

// DeleteConfig удаляет конфиг
// @Summary Удаление конфига OnlineConf
// @Tags onlineconf-admin
// @Param scope path string true "Скоуп"
// @Param key path string true "Ключ"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 204 "Конфиг удалён"
// @Failure 404 {object} response.ErrorResponse "Конфиг не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/configs/{scope}/{key} [delete]
func (h *ConfigsHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	vars := mux.Vars(r)

	if err := h.configs.DeleteConfig(r.Context(), actor, vars["scope"], vars["key"]); err != nil {
		handleError(r.Context(), h.logger, w, "delete config", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

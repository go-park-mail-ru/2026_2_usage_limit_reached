package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type FlagsHandler struct {
	flags  AdminUsecase
	logger *slog.Logger
}

func NewFlagsHandler(flags AdminUsecase, logger *slog.Logger) *FlagsHandler {
	return &FlagsHandler{flags: flags, logger: logger}
}

// ListFlags отдаёт флаги скоупа с поиском по названию
// @Summary Список флагов OnlineConf
// @Description Поиск по названию (ILIKE) и фильтр по скоупу; флаги приходят с force-списками
// @Tags onlineconf-admin
// @Produce json
// @Param scope query string false "Фильтр по скоупу"
// @Param q query string false "Поиск по названию"
// @Success 200 {array} dto.FlagResponse "Флаги"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/flags [get]
func (h *FlagsHandler) ListFlags(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	flags, err := h.flags.ListFlags(r.Context(), query.Get("scope"), query.Get("q"))
	if err != nil {
		handleError(r.Context(), h.logger, w, "list flags", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, flags)
}

// CreateFlag создаёт флаг
// @Summary Создание флага OnlineConf
// @Tags onlineconf-admin
// @Accept json
// @Produce json
// @Param request body dto.FlagRequest true "Новый флаг"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 201 {object} dto.FlagResponse "Флаг создан"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 404 {object} response.ErrorResponse "Скоуп не найден"
// @Failure 409 {object} response.ErrorResponse "Флаг уже существует"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/flags [post]
func (h *FlagsHandler) CreateFlag(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.FlagRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(r.Context(), h.logger, w, "create flag", err)
		return
	}

	if err := req.Validate(); err != nil {
		handleError(r.Context(), h.logger, w, "create flag", err)
		return
	}

	flag, err := h.flags.CreateFlag(r.Context(), actor, req.Scope, req.Key, req.Description, req.RolloutPercent)
	if err != nil {
		handleError(r.Context(), h.logger, w, "create flag", err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, flag)
}

// GetFlag отдаёт флаг по скоупу и ключу
// @Summary Чтение флага OnlineConf
// @Tags onlineconf-admin
// @Produce json
// @Param scope path string true "Скоуп"
// @Param key path string true "Ключ"
// @Success 200 {object} dto.FlagResponse "Флаг с force-списками"
// @Failure 404 {object} response.ErrorResponse "Флаг не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/flags/{scope}/{key} [get]
func (h *FlagsHandler) GetFlag(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	flag, err := h.flags.GetFlag(r.Context(), vars["scope"], vars["key"])
	if err != nil {
		handleError(r.Context(), h.logger, w, "get flag", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, flag)
}

// UpdateRollout меняет процентную раскатку
// @Summary Изменение раскатки флага
// @Description Новая раскатка применяется у потребителей не позднее чем через минуту; включённые пользователи не выпадают при росте процента
// @Tags onlineconf-admin
// @Accept json
// @Produce json
// @Param scope path string true "Скоуп"
// @Param key path string true "Ключ"
// @Param request body dto.RolloutRequest true "Новый процент"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 200 {object} dto.FlagResponse "Раскатка обновлена"
// @Failure 400 {object} response.ErrorResponse "Процент вне диапазона 0..100"
// @Failure 404 {object} response.ErrorResponse "Флаг не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/flags/{scope}/{key}/rollout [put]
func (h *FlagsHandler) UpdateRollout(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.RolloutRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(r.Context(), h.logger, w, "update rollout", err)
		return
	}

	if err := req.Validate(); err != nil {
		handleError(r.Context(), h.logger, w, "update rollout", err)
		return
	}

	vars := mux.Vars(r)

	flag, err := h.flags.UpdateRollout(r.Context(), actor, vars["scope"], vars["key"], req.Percent)
	if err != nil {
		handleError(r.Context(), h.logger, w, "update rollout", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, flag)
}

// SetForce принудительно включает/исключает пользователя
// @Summary Принудительное включение/исключение юзера
// @Description force_off приоритетнее force_on и раскатки (закреплено PK таблицы)
// @Tags onlineconf-admin
// @Accept json
// @Produce json
// @Param scope path string true "Скоуп"
// @Param key path string true "Ключ"
// @Param request body dto.ForceRequest true "Пользователь и режим"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 204 "Переопределение сохранено"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 404 {object} response.ErrorResponse "Флаг не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/flags/{scope}/{key}/force [post]
func (h *FlagsHandler) SetForce(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.ForceRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(r.Context(), h.logger, w, "set force", err)
		return
	}

	if err := req.Validate(); err != nil {
		handleError(r.Context(), h.logger, w, "set force", err)
		return
	}

	vars := mux.Vars(r)

	if err := h.flags.SetForce(r.Context(), actor, vars["scope"], vars["key"], req.UserID, req.Force); err != nil {
		handleError(r.Context(), h.logger, w, "set force", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveForce снимает переопределение пользователя
// @Summary Снятие переопределения юзера
// @Tags onlineconf-admin
// @Param scope path string true "Скоуп"
// @Param key path string true "Ключ"
// @Param user_id path string true "UUID пользователя"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 204 "Переопределение снято"
// @Failure 404 {object} response.ErrorResponse "Флаг или переопределение не найдены"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/flags/{scope}/{key}/force/{user_id} [delete]
func (h *FlagsHandler) RemoveForce(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	vars := mux.Vars(r)

	userID, err := uuid.Parse(vars["user_id"])
	if err != nil {
		handleError(r.Context(), h.logger, w, "remove force", err)
		return
	}

	if err := h.flags.RemoveForce(r.Context(), actor, vars["scope"], vars["key"], userID); err != nil {
		handleError(r.Context(), h.logger, w, "remove force", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteFlag удаляет флаг
// @Summary Удаление флага OnlineConf
// @Tags onlineconf-admin
// @Param scope path string true "Скоуп"
// @Param key path string true "Ключ"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 204 "Флаг удалён"
// @Failure 404 {object} response.ErrorResponse "Флаг не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/flags/{scope}/{key} [delete]
func (h *FlagsHandler) DeleteFlag(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	vars := mux.Vars(r)

	if err := h.flags.DeleteFlag(r.Context(), actor, vars["scope"], vars["key"]); err != nil {
		handleError(r.Context(), h.logger, w, "delete flag", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CheckFlag проверяет состояние тоггла для конкретного пользователя
// @Summary Проверка юзера на флаге (бонус)
// @Description Результирующее состояние с учётом rollout и персональных настроек; reason объясняет, почему включено/выключено
// @Tags onlineconf-admin
// @Accept json
// @Produce json
// @Param request body dto.CheckRequest true "Скоуп, ключ и пользователь"
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 200 {object} dto.CheckResponse "Результат вычисления"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /onlineconf/admin/flags/check [post]
func (h *FlagsHandler) CheckFlag(w http.ResponseWriter, r *http.Request) {
	var req dto.CheckRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(r.Context(), h.logger, w, "check flag", err)
		return
	}

	if err := req.Validate(); err != nil {
		handleError(r.Context(), h.logger, w, "check flag", err)
		return
	}

	decision, err := h.flags.CheckFlag(r.Context(), req.Scope, req.Key, req.UserID)
	if err != nil {
		handleError(r.Context(), h.logger, w, "check flag", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, decision)
}

// Package dto — транспортные форматы admin API и /front OnlineConf.
package dto

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	contracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/onlineconf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
)

// AccessResponse — ответ GET /api/onlineconf/admin/access.
// 401/403 отдаются мидлварями (AuthMiddleware / PermissionMiddleware)
// с телами {"error":"unauthorized"} / {"error":"forbidden"} — контракт PTN-71.
type AccessResponse struct {
	Allowed bool `json:"allowed" example:"true"`
}

type ScopeResponse struct {
	Name        string `json:"name" example:"front"`
	Description string `json:"description" example:"Настройки и флаги продуктового фронтенда"`
}

func ToScopeResponse(s models.Scope) ScopeResponse {
	return ScopeResponse{Name: s.Name, Description: s.Description}
}

type ScopeRequest struct {
	Name        string `json:"name" example:"front"`
	Description string `json:"description" example:"Новый скоуп"`
}

func (r ScopeRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return validator.ErrValidation
	}
	return nil
}

type ConfigResponse struct {
	Scope       string    `json:"scope" example:"front"`
	Key         string    `json:"key" example:"payment.min_amount_minor"`
	Value       string    `json:"value" example:"300"`
	ValueType   string    `json:"value_type" example:"scalar"`
	Description string    `json:"description" example:"Минимальная сумма доната в копейках"`
	UpdatedBy   *uuid.UUID `json:"updated_by,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func ToConfigResponse(c models.Config) ConfigResponse {
	return ConfigResponse{
		Scope:       c.Scope,
		Key:         c.Key,
		Value:       c.Value,
		ValueType:   c.ValueType,
		Description: c.Description,
		UpdatedBy:   c.UpdatedBy,
		UpdatedAt:   c.UpdatedAt,
	}
}

// ConfigRequest — создание конфига. Value — строка, содержащая JSON:
// "300" для scalar, сериализованный JSON для value_type json/array.
type ConfigRequest struct {
	Scope       string `json:"scope" example:"front"`
	Key         string `json:"key" example:"payment.min_amount_minor"`
	Value       string `json:"value" example:"300"`
	ValueType   string `json:"value_type" example:"scalar"`
	Description string `json:"description" example:"Минимальная сумма доната"`
}

func (r ConfigRequest) Validate() error {
	if strings.TrimSpace(r.Scope) == "" ||
		strings.TrimSpace(r.Key) == "" ||
		!isValidValueType(r.ValueType) ||
		!json.Valid([]byte(r.Value)) {
		return validator.ErrValidation
	}
	return nil
}

// ValidateValue проверяет только значение: используется при обновлении,
// где scope и key приходят в пути, а не в теле запроса.
func (r ConfigRequest) ValidateValue() error {
	if !isValidValueType(r.ValueType) || !json.Valid([]byte(r.Value)) {
		return validator.ErrValidation
	}
	return nil
}

func isValidValueType(t string) bool {
	switch t {
	case models.ValueTypeScalar, models.ValueTypeArray, models.ValueTypeJSON:
		return true
	default:
		return false
	}
}

type FlagResponse struct {
	Scope           string    `json:"scope" example:"content"`
	Key             string    `json:"key" example:"feed_attachment_filter"`
	Description     string    `json:"description" example:"Фильтр ленты по типу вложений"`
	RolloutPercent  int       `json:"rollout_percent" example:"30"`
	ForceOnUserIDs  []string  `json:"force_on_user_ids"`
	ForceOffUserIDs []string  `json:"force_off_user_ids"`
	UpdatedBy       *uuid.UUID `json:"updated_by,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func ToFlagResponse(f models.Flag) FlagResponse {
	on := make([]string, 0, len(f.ForceOn))
	for _, id := range f.ForceOn {
		on = append(on, id.String())
	}
	off := make([]string, 0, len(f.ForceOff))
	for _, id := range f.ForceOff {
		off = append(off, id.String())
	}

	return FlagResponse{
		Scope:           f.Scope,
		Key:             f.Key,
		Description:     f.Description,
		RolloutPercent:  int(f.RolloutPercent),
		ForceOnUserIDs:  on,
		ForceOffUserIDs: off,
		UpdatedBy:       f.UpdatedBy,
		UpdatedAt:       f.UpdatedAt,
	}
}

type FlagRequest struct {
	Scope          string `json:"scope" example:"content"`
	Key            string `json:"key" example:"feed_attachment_filter"`
	Description    string `json:"description" example:"Фильтр ленты по типу вложений"`
	RolloutPercent int    `json:"rollout_percent" example:"0"`
}

func (r FlagRequest) Validate() error {
	if strings.TrimSpace(r.Scope) == "" || strings.TrimSpace(r.Key) == "" {
		return validator.ErrValidation
	}
	if r.RolloutPercent < 0 || r.RolloutPercent > 100 {
		return validator.ErrValidation
	}
	return nil
}

type RolloutRequest struct {
	Percent int `json:"percent" example:"30"`
}

func (r RolloutRequest) Validate() error {
	if r.Percent < 0 || r.Percent > 100 {
		return validator.ErrValidation
	}
	return nil
}

type ForceRequest struct {
	UserID uuid.UUID `json:"user_id"`
	Force  string    `json:"force" example:"on"`
}

func (r ForceRequest) Validate() error {
	switch r.Force {
	case "on", "off":
		return nil
	default:
		return validator.ErrValidation
	}
}

type CheckRequest struct {
	Scope  string    `json:"scope" example:"content"`
	Key    string    `json:"key" example:"feed_attachment_filter"`
	UserID uuid.UUID `json:"user_id"`
}

func (r CheckRequest) Validate() error {
	if strings.TrimSpace(r.Scope) == "" || strings.TrimSpace(r.Key) == "" {
		return validator.ErrValidation
	}
	return nil
}

type CheckResponse struct {
	Enabled bool               `json:"enabled" example:"true"`
	Reason  contracts.Reason `json:"reason" example:"force_on"`
}

type HistoryResponse struct {
	ID         int64           `json:"id"`
	ActorID    *uuid.UUID      `json:"actor_id,omitempty"`
	EntityType string          `json:"entity_type" example:"flag"`
	EntityID   uuid.UUID       `json:"entity_id"`
	Action     string          `json:"action" example:"update"`
	OldValue   json.RawMessage `json:"old_value,omitempty"`
	NewValue   json.RawMessage `json:"new_value,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

func ToHistoryResponse(e models.HistoryEntry) HistoryResponse {
	resp := HistoryResponse{
		ID:         e.ID,
		ActorID:    e.ActorID,
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		Action:     e.Action,
		CreatedAt:  e.CreatedAt,
	}
	if e.OldValue != nil {
		resp.OldValue = json.RawMessage(*e.OldValue)
	}
	if e.NewValue != nil {
		resp.NewValue = json.RawMessage(*e.NewValue)
	}
	return resp
}

// HistoryFilter — фильтр списка истории. EntityType нужен только вместе
// со Scope/Key: без них история отдается целиком.
type HistoryFilter struct {
	EntityType string `json:"entity_type"`
	Scope      string `json:"scope"`
	Key        string `json:"key"`
}

// HistoryPage — страница истории. Cursor присутствует только тогда,
// когда есть следующая страница.
type HistoryPage struct {
	Items  []HistoryResponse `json:"items"`
	Cursor *string           `json:"cursor,omitempty"`
}

// HistoryCursor — курсор страниц истории по конвенции pkg/cursor.
type HistoryCursor struct {
	ID int64 `json:"id"`
}

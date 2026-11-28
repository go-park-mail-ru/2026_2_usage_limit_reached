// Package usecase — бизнес-логика OnlineConf: валидация мутаций,
// проверка прав и вычисление флагов.
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	onlineconfclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/onlineconf"
	contracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/onlineconf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/models"
)

// Сентинелы ошибок для delivery: транспорт не должен импортировать models,
// поэтому usecase переэкспортирует доменные ошибки (конвенция репо).
var (
	ErrScopeNotFound  = models.ErrScopeNotFound
	ErrScopeExists    = models.ErrScopeExists
	ErrConfigNotFound = models.ErrConfigNotFound
	ErrConfigExists   = models.ErrConfigExists
	ErrFlagNotFound   = models.ErrFlagNotFound
	ErrFlagExists     = models.ErrFlagExists
	ErrInvalidValue   = models.ErrInvalidValue
)

type Repository interface {
	GetSnapshot(ctx context.Context, scopes []string, knownRevision int64) (models.Snapshot, error)
	CanManageOnlineConf(ctx context.Context, userID uuid.UUID) (bool, error)

	ListScopes(ctx context.Context) ([]models.Scope, error)
	CreateScope(ctx context.Context, actor uuid.UUID, scope models.Scope) (*models.Scope, error)

	ListConfigs(ctx context.Context, scope, search string) ([]models.Config, error)
	GetConfig(ctx context.Context, scope, key string) (*models.Config, error)
	CreateConfig(ctx context.Context, actor uuid.UUID, cfg models.Config) (*models.Config, error)
	UpdateConfig(ctx context.Context, actor uuid.UUID, scope, key string, cfg models.Config) (*models.Config, error)
	DeleteConfig(ctx context.Context, actor uuid.UUID, scope, key string) error

	ListFlags(ctx context.Context, scope, search string) ([]models.Flag, error)
	GetFlag(ctx context.Context, scope, key string) (*models.Flag, error)
	CreateFlag(ctx context.Context, actor uuid.UUID, flag models.Flag) (*models.Flag, error)
	UpdateRollout(ctx context.Context, actor uuid.UUID, scope, key string, percent int16) (*models.Flag, error)
	SetForce(ctx context.Context, actor uuid.UUID, scope, key string, userID uuid.UUID, force string) error
	RemoveForce(ctx context.Context, actor uuid.UUID, scope, key string, userID uuid.UUID) error
	DeleteFlag(ctx context.Context, actor uuid.UUID, scope, key string) error

	ListHistory(ctx context.Context, filter models.HistoryFilter, cursorID *int64, limit int) ([]models.HistoryEntry, *int64, error)
}

type Usecase struct {
	repo   Repository
	logger *slog.Logger
}

func New(repo Repository, logger *slog.Logger) *Usecase {
	return &Usecase{repo: repo, logger: logger}
}

// CanManage — право управлять OnlineConf: живой SELECT роли и статуса из
// БД (дизайн, решение 16). Токен отвечает за идентификацию, БД — за то,
// что разрешено прямо сейчас. Удовлетворяет middleware.PermissionChecker.
func (uc *Usecase) CanManage(ctx context.Context, userID uuid.UUID) (bool, error) {
	return uc.repo.CanManageOnlineConf(ctx, userID)
}

// GetSnapshot — срез для сервисов-потребителей по gRPC: сырые данные,
// ноль вычислений «под юзера».
func (uc *Usecase) GetSnapshot(ctx context.Context, scopes []string, knownRevision int64) (models.Snapshot, error) {
	return uc.repo.GetSnapshot(ctx, scopes, knownRevision)
}

func (uc *Usecase) ListScopes(ctx context.Context) ([]dto.ScopeResponse, error) {
	scopes, err := uc.repo.ListScopes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list scopes: %w", err)
	}

	resp := make([]dto.ScopeResponse, 0, len(scopes))
	for _, scope := range scopes {
		resp = append(resp, dto.ToScopeResponse(scope))
	}

	return resp, nil
}

func (uc *Usecase) CreateScope(ctx context.Context, actor uuid.UUID, name, description string) (*dto.ScopeResponse, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("create scope: empty name")
	}

	scope, err := uc.repo.CreateScope(ctx, actor, models.Scope{Name: name, Description: description})
	if err != nil {
		return nil, fmt.Errorf("create scope: %w", err)
	}

	resp := dto.ToScopeResponse(*scope)
	return &resp, nil
}

func (uc *Usecase) ListConfigs(ctx context.Context, scope, search string) ([]dto.ConfigResponse, error) {
	configs, err := uc.repo.ListConfigs(ctx, scope, search)
	if err != nil {
		return nil, fmt.Errorf("list configs: %w", err)
	}

	resp := make([]dto.ConfigResponse, 0, len(configs))
	for _, cfg := range configs {
		resp = append(resp, dto.ToConfigResponse(cfg))
	}

	return resp, nil
}

func (uc *Usecase) GetConfig(ctx context.Context, scope, key string) (*dto.ConfigResponse, error) {
	cfg, err := uc.repo.GetConfig(ctx, scope, key)
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	resp := dto.ToConfigResponse(*cfg)
	return &resp, nil
}

func (uc *Usecase) CreateConfig(ctx context.Context, actor uuid.UUID, scope, key, value, valueType, description string) (*dto.ConfigResponse, error) {
	if err := validateValue(value, valueType); err != nil {
		return nil, fmt.Errorf("create config: %w", err)
	}

	cfg, err := uc.repo.CreateConfig(ctx, actor, models.Config{
		Scope: strings.TrimSpace(scope),
		Key:   strings.TrimSpace(key),
		Value: value, ValueType: valueType,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("create config: %w", err)
	}

	resp := dto.ToConfigResponse(*cfg)
	return &resp, nil
}

func (uc *Usecase) UpdateConfig(ctx context.Context, actor uuid.UUID, scope, key, value, valueType, description string) (*dto.ConfigResponse, error) {
	if err := validateValue(value, valueType); err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	cfg, err := uc.repo.UpdateConfig(ctx, actor, scope, key, models.Config{
		Value: value, ValueType: valueType, Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	resp := dto.ToConfigResponse(*cfg)
	return &resp, nil
}

func (uc *Usecase) DeleteConfig(ctx context.Context, actor uuid.UUID, scope, key string) error {
	if err := uc.repo.DeleteConfig(ctx, actor, scope, key); err != nil {
		return fmt.Errorf("delete config: %w", err)
	}
	return nil
}

func (uc *Usecase) ListFlags(ctx context.Context, scope, search string) ([]dto.FlagResponse, error) {
	flags, err := uc.repo.ListFlags(ctx, scope, search)
	if err != nil {
		return nil, fmt.Errorf("list flags: %w", err)
	}

	resp := make([]dto.FlagResponse, 0, len(flags))
	for _, flag := range flags {
		resp = append(resp, dto.ToFlagResponse(flag))
	}

	return resp, nil
}

func (uc *Usecase) GetFlag(ctx context.Context, scope, key string) (*dto.FlagResponse, error) {
	flag, err := uc.repo.GetFlag(ctx, scope, key)
	if err != nil {
		return nil, fmt.Errorf("get flag: %w", err)
	}

	resp := dto.ToFlagResponse(*flag)
	return &resp, nil
}

func (uc *Usecase) CreateFlag(ctx context.Context, actor uuid.UUID, scope, key, description string, rolloutPercent int) (*dto.FlagResponse, error) {
	if rolloutPercent < 0 || rolloutPercent > 100 {
		return nil, fmt.Errorf("create flag: invalid rollout percent %d", rolloutPercent)
	}

	flag, err := uc.repo.CreateFlag(ctx, actor, models.Flag{
		Scope:          strings.TrimSpace(scope),
		Key:            strings.TrimSpace(key),
		Description:    description,
		RolloutPercent: int16(rolloutPercent),
	})
	if err != nil {
		return nil, fmt.Errorf("create flag: %w", err)
	}

	resp := dto.ToFlagResponse(*flag)
	return &resp, nil
}

func (uc *Usecase) UpdateRollout(ctx context.Context, actor uuid.UUID, scope, key string, percent int) (*dto.FlagResponse, error) {
	if percent < 0 || percent > 100 {
		return nil, fmt.Errorf("update rollout: invalid percent %d", percent)
	}

	flag, err := uc.repo.UpdateRollout(ctx, actor, scope, key, int16(percent))
	if err != nil {
		return nil, fmt.Errorf("update rollout: %w", err)
	}

	resp := dto.ToFlagResponse(*flag)
	return &resp, nil
}

func (uc *Usecase) SetForce(ctx context.Context, actor uuid.UUID, scope, key string, userID uuid.UUID, force string) error {
	if force != "on" && force != "off" {
		return fmt.Errorf("set force: invalid force value %q", force)
	}

	if err := uc.repo.SetForce(ctx, actor, scope, key, userID, force); err != nil {
		return fmt.Errorf("set force: %w", err)
	}
	return nil
}

func (uc *Usecase) RemoveForce(ctx context.Context, actor uuid.UUID, scope, key string, userID uuid.UUID) error {
	if err := uc.repo.RemoveForce(ctx, actor, scope, key, userID); err != nil {
		return fmt.Errorf("remove force: %w", err)
	}
	return nil
}

func (uc *Usecase) DeleteFlag(ctx context.Context, actor uuid.UUID, scope, key string) error {
	if err := uc.repo.DeleteFlag(ctx, actor, scope, key); err != nil {
		return fmt.Errorf("delete flag: %w", err)
	}
	return nil
}

// CheckFlag вычисляет результирующее состояние тоггла для конкретного
// пользователя с учётом раскатки и персональных настроек (бонус 2).
// Отсутствующий флаг — не ошибка, а reason flag_not_found.
func (uc *Usecase) CheckFlag(ctx context.Context, scope, key string, userID uuid.UUID) (*dto.CheckResponse, error) {
	flag, err := uc.repo.GetFlag(ctx, scope, key)
	if errors.Is(err, models.ErrFlagNotFound) {
		return &dto.CheckResponse{Enabled: false, Reason: contracts.ReasonFlagNotFound}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("check flag: %w", err)
	}

	decision := onlineconfclient.Evaluate(toContractsFlag(*flag), userID)
	return &dto.CheckResponse{Enabled: decision.Enabled, Reason: decision.Reason}, nil
}

// ListHistory отдаёт страницу журнала. Кодированием курсора занимается
// delivery (транспортный формат), здесь — только доменные данные.
func (uc *Usecase) ListHistory(ctx context.Context, filter dto.HistoryFilter, cursorID *int64, limit int) ([]dto.HistoryResponse, *int64, error) {
	entries, nextCursor, err := uc.repo.ListHistory(ctx, models.HistoryFilter{
		EntityType: filter.EntityType,
		Scope:      filter.Scope,
		Key:        filter.Key,
	}, cursorID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("list history: %w", err)
	}

	resp := make([]dto.HistoryResponse, 0, len(entries))
	for _, entry := range entries {
		resp = append(resp, dto.ToHistoryResponse(entry))
	}

	return resp, nextCursor, nil
}

// validateValue проверяет соответствие JSON-значения заявленному формату:
// scalar — число/строка/булево, array — JSON-массив, json — JSON-объект.
// Синтаксис уже проверен на уровне dto (json.Valid) — здесь семантика.
func validateValue(value, valueType string) error {
	var raw any
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return models.ErrInvalidValue
	}

	switch valueType {
	case models.ValueTypeScalar:
		if _, ok := raw.(map[string]any); ok {
			return models.ErrInvalidValue
		}
		if _, ok := raw.([]any); ok {
			return models.ErrInvalidValue
		}
	case models.ValueTypeArray:
		if _, ok := raw.([]any); !ok {
			return models.ErrInvalidValue
		}
	case models.ValueTypeJSON:
		if _, ok := raw.(map[string]any); !ok {
			return models.ErrInvalidValue
		}
	default:
		return models.ErrInvalidValue
	}

	return nil
}

func toContractsFlag(f models.Flag) contracts.Flag {
	forceOn := make(map[uuid.UUID]struct{}, len(f.ForceOn))
	for _, id := range f.ForceOn {
		forceOn[id] = struct{}{}
	}

	forceOff := make(map[uuid.UUID]struct{}, len(f.ForceOff))
	for _, id := range f.ForceOff {
		forceOff[id] = struct{}{}
	}

	return contracts.Flag{
		Scope:          f.Scope,
		Key:            f.Key,
		RolloutPercent: uint32(f.RolloutPercent),
		ForceOn:        forceOn,
		ForceOff:       forceOff,
	}
}

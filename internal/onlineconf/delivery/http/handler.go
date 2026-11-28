// Package handlers — HTTP delivery OnlineConf: админ-API, /access и /front.
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/dto"
	onlineconfuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type AdminUsecase interface {
	ListScopes(ctx context.Context) ([]dto.ScopeResponse, error)
	CreateScope(ctx context.Context, actor uuid.UUID, name, description string) (*dto.ScopeResponse, error)

	ListConfigs(ctx context.Context, scope, search string) ([]dto.ConfigResponse, error)
	GetConfig(ctx context.Context, scope, key string) (*dto.ConfigResponse, error)
	CreateConfig(ctx context.Context, actor uuid.UUID, scope, key, value, valueType, description string) (*dto.ConfigResponse, error)
	UpdateConfig(ctx context.Context, actor uuid.UUID, scope, key, value, valueType, description string) (*dto.ConfigResponse, error)
	DeleteConfig(ctx context.Context, actor uuid.UUID, scope, key string) error

	ListFlags(ctx context.Context, scope, search string) ([]dto.FlagResponse, error)
	GetFlag(ctx context.Context, scope, key string) (*dto.FlagResponse, error)
	CreateFlag(ctx context.Context, actor uuid.UUID, scope, key, description string, rolloutPercent int) (*dto.FlagResponse, error)
	UpdateRollout(ctx context.Context, actor uuid.UUID, scope, key string, percent int) (*dto.FlagResponse, error)
	SetForce(ctx context.Context, actor uuid.UUID, scope, key string, userID uuid.UUID, force string) error
	RemoveForce(ctx context.Context, actor uuid.UUID, scope, key string, userID uuid.UUID) error
	DeleteFlag(ctx context.Context, actor uuid.UUID, scope, key string) error
	CheckFlag(ctx context.Context, scope, key string, userID uuid.UUID) (*dto.CheckResponse, error)

	ListHistory(ctx context.Context, filter dto.HistoryFilter, cursorID *int64, limit int) ([]dto.HistoryResponse, *int64, error)
}

func handleError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, action string, err error) {
	logError(ctx, logger, action, err)

	switch {
	case errors.Is(err, validator.ErrValidation), errors.Is(err, onlineconfuc.ErrInvalidValue):
		response.ErrorBadRequest(w)
	case errors.Is(err, onlineconfuc.ErrScopeExists),
		errors.Is(err, onlineconfuc.ErrConfigExists),
		errors.Is(err, onlineconfuc.ErrFlagExists):
		response.ErrorConflict(w)
	case errors.Is(err, onlineconfuc.ErrScopeNotFound),
		errors.Is(err, onlineconfuc.ErrConfigNotFound),
		errors.Is(err, onlineconfuc.ErrFlagNotFound):
		response.ErrorNotFound(w)
	default:
		response.ErrorInternal(w)
	}
}

func logError(ctx context.Context, logger *slog.Logger, action string, err error) {
	if logger == nil {
		return
	}

	logger.ErrorContext(ctx, "onlineconf http error",
		slog.String("handler", action),
		slog.String("error", err.Error()),
	)
}

// actorFromContext достает пользователя, проставленного AuthMiddleware:
// идентификация из токена, права проверяются живым SELECT'ом из БД.
func actorFromContext(r *http.Request) (uuid.UUID, bool) {
	user, ok := middleware.UserInfoFromContext(r.Context())
	if !ok || user == nil {
		return uuid.Nil, false
	}

	return user.ID, true
}

// userFromContext как actorFromContext, но для маршрутов с OptionalAuth:
// аноним возвращает uuid.Nil (все флаги для него выключены).
func userFromContext(r *http.Request) uuid.UUID {
	if user, ok := middleware.UserInfoFromContext(r.Context()); ok && user != nil {
		return user.ID
	}

	return uuid.Nil
}

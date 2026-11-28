// Package repository — доступ к таблицам oc_* OnlineConf.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
)

// mutationLockKey — ключ advisory-блокировки, сериализующей все мутации.
// Сериализация обязательна: ревизия = max(oc_history.id), а sequence выдаёт
// id при INSERT, не при COMMIT — без блокировки поздно закоммитившаяся
// мутация с меньшим id навсегда прячется за max(id) (дизайн, решение 6).
const mutationLockKey = "onlineconf:mutation"

type Repository struct {
	pool   postgres.DB
	logger *slog.Logger
}

func NewRepository(db *postgres.DBClient, logger *slog.Logger) *Repository {
	return &Repository{pool: db.Pool, logger: logger}
}

type querier interface {
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) pgx.Row
}

// GetSnapshot читает срез скоупов вместе с ревизией. Порядок критичен:
// ревизия и данные читаются в ОДНОЙ транзакции REPEATABLE READ, причём
// max(oc_history.id) — ДО строк данных. Обратный порядок даёт пару
// «ревизия N, данные N-1», которая теряет изменение до следующей мутации.
// known_revision = 0 — первая загрузка клиента: всегда полный срез.
func (r *Repository) GetSnapshot(ctx context.Context, scopes []string, knownRevision int64) (models.Snapshot, error) {
	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get snapshot"),
		slog.Int("scopes", len(scopes)),
	)

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return models.Snapshot{}, fmt.Errorf("begin snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op после успешного commit

	var revision int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(id), 0) FROM oc_history`).Scan(&revision); err != nil {
		return models.Snapshot{}, fmt.Errorf("read revision: %w", err)
	}

	if knownRevision > 0 && knownRevision == revision {
		if err := tx.Commit(ctx); err != nil {
			return models.Snapshot{}, fmt.Errorf("commit snapshot tx: %w", err)
		}
		return models.Snapshot{Revision: revision, NotModified: true}, nil
	}

	configs, err := r.queryConfigs(ctx, tx, scopes)
	if err != nil {
		return models.Snapshot{}, err
	}

	flags, err := r.queryFlags(ctx, tx, scopes)
	if err != nil {
		return models.Snapshot{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Snapshot{}, fmt.Errorf("commit snapshot tx: %w", err)
	}

	return models.Snapshot{Revision: revision, Configs: configs, Flags: flags}, nil
}

// CanManageOnlineConf проверяет право управления живым SELECT'ом роли и
// статуса из "user" (дизайн, решение 16): токен идентифицирует, БД решает.
func (r *Repository) CanManageOnlineConf(ctx context.Context, userID uuid.UUID) (bool, error) {
	const query = `
		SELECT role, status
		FROM "user"
		WHERE id = $1
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "can manage onlineconf"),
		slog.String("userID", userID.String()),
	)

	var role, status string
	err := r.pool.QueryRow(ctx, query, userID).Scan(&role, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("can manage onlineconf error: %w", err)
	}

	return role == models.RoleAdmin && status == models.StatusActive, nil
}

func (r *Repository) ListScopes(ctx context.Context) ([]models.Scope, error) {
	const query = `
		SELECT id, name, description, created_at, updated_at
		FROM oc_scope
		ORDER BY name
	`

	r.logger.DebugContext(ctx, "executing query", slog.String("op", "list scopes"))

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list scopes error: %w", err)
	}
	defer rows.Close()

	scopes := make([]models.Scope, 0)
	for rows.Next() {
		var s models.Scope
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan scope: %w", err)
		}
		scopes = append(scopes, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list scopes rows: %w", err)
	}

	return scopes, nil
}

func (r *Repository) CreateScope(ctx context.Context, actor uuid.UUID, scope models.Scope) (*models.Scope, error) {
	var created models.Scope

	err := r.withMutation(ctx, func(tx pgx.Tx) error {
		const query = `
			INSERT INTO oc_scope (name, description)
			VALUES ($1, $2)
			RETURNING id, created_at, updated_at
		`

		r.logger.DebugContext(ctx, "executing query",
			slog.String("op", "create scope"),
			slog.String("name", scope.Name),
		)

		err := tx.QueryRow(ctx, query, scope.Name, scope.Description).Scan(&created.ID, &created.CreatedAt, &created.UpdatedAt)
		if err != nil {
			if errors.Is(postgres.ParseError(err), postgres.ErrUniqViolation) {
				return models.ErrScopeExists
			}
			return fmt.Errorf("create scope error: %w", err)
		}

		created.Name, created.Description = scope.Name, scope.Description
		return r.insertHistory(ctx, tx, actor, "scope", created.ID, "create", nil, scope)
	})
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *Repository) ListConfigs(ctx context.Context, scope, search string) ([]models.Config, error) {
	query, args := buildListQuery(`
		SELECT id, scope, key, value::text, value_type, description, updated_by, created_at, updated_at
		FROM oc_config
	`, scope, search)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list configs error: %w", err)
	}
	defer rows.Close()

	configs := make([]models.Config, 0)
	for rows.Next() {
		c, err := scanConfigRow(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list configs rows: %w", err)
	}

	return configs, nil
}

func (r *Repository) GetConfig(ctx context.Context, scope, key string) (*models.Config, error) {
	const query = `
		SELECT id, scope, key, value::text, value_type, description, updated_by, created_at, updated_at
		FROM oc_config
		WHERE scope = $1 AND key = $2
	`

	return r.queryOneConfig(ctx, r.pool, query, scope, key)
}

func (r *Repository) CreateConfig(ctx context.Context, actor uuid.UUID, cfg models.Config) (*models.Config, error) {
	var created models.Config

	err := r.withMutation(ctx, func(tx pgx.Tx) error {
		const query = `
			INSERT INTO oc_config (scope, key, value, value_type, description, updated_by)
			VALUES ($1, $2, $3::jsonb, $4, $5, $6)
			RETURNING id, created_at, updated_at
		`

		r.logger.DebugContext(ctx, "executing query",
			slog.String("op", "create config"),
			slog.String("scope", cfg.Scope),
			slog.String("key", cfg.Key),
		)

		err := tx.QueryRow(ctx, query, cfg.Scope, cfg.Key, cfg.Value, cfg.ValueType, cfg.Description, actor).
			Scan(&created.ID, &created.CreatedAt, &created.UpdatedAt)
		if err != nil {
			parsed := postgres.ParseError(err)
			switch {
			case errors.Is(parsed, postgres.ErrUniqViolation):
				return models.ErrConfigExists
			case errors.Is(parsed, postgres.ErrFKViolation):
				return models.ErrScopeNotFound
			}
			return fmt.Errorf("create config error: %w", err)
		}

		created.Scope, created.Key = cfg.Scope, cfg.Key
		created.Value, created.ValueType = cfg.Value, cfg.ValueType
		created.Description, created.UpdatedBy = cfg.Description, &actor

		return r.insertHistory(ctx, tx, actor, "config", created.ID, "create", nil, configHistoryValue(cfg))
	})
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *Repository) UpdateConfig(ctx context.Context, actor uuid.UUID, scope, key string, cfg models.Config) (*models.Config, error) {
	var updated models.Config

	err := r.withMutation(ctx, func(tx pgx.Tx) error {
		const lockQuery = `
			SELECT id, scope, key, value::text, value_type, description, updated_by, created_at, updated_at
			FROM oc_config
			WHERE scope = $1 AND key = $2
			FOR UPDATE
		`

		old, err := r.queryOneConfig(ctx, tx, lockQuery, scope, key)
		if err != nil {
			return err
		}

		const query = `
			UPDATE oc_config
			SET value = $2::jsonb, value_type = $3, description = $4, updated_by = $5
			WHERE id = $1
			RETURNING updated_at
		`

		r.logger.DebugContext(ctx, "executing query",
			slog.String("op", "update config"),
			slog.String("scope", scope),
			slog.String("key", key),
		)

		if err := tx.QueryRow(ctx, query, old.ID, cfg.Value, cfg.ValueType, cfg.Description, actor).Scan(&updated.UpdatedAt); err != nil {
			return fmt.Errorf("update config error: %w", err)
		}

		updated.ID, updated.Scope, updated.Key = old.ID, old.Scope, old.Key
		updated.Value, updated.ValueType = cfg.Value, cfg.ValueType
		updated.Description, updated.UpdatedBy = cfg.Description, &actor
		updated.CreatedAt = old.CreatedAt

		return r.insertHistory(ctx, tx, actor, "config", old.ID, "update", configHistoryValue(*old), configHistoryValue(updated))
	})
	if err != nil {
		return nil, err
	}

	return &updated, nil
}

func (r *Repository) DeleteConfig(ctx context.Context, actor uuid.UUID, scope, key string) error {
	return r.withMutation(ctx, func(tx pgx.Tx) error {
		const query = `
			DELETE FROM oc_config
			WHERE scope = $1 AND key = $2
			RETURNING id, value::text, value_type, description
		`

		r.logger.DebugContext(ctx, "executing query",
			slog.String("op", "delete config"),
			slog.String("scope", scope),
			slog.String("key", key),
		)

		var (
			id          uuid.UUID
			value       string
			valueType   string
			description string
		)
		err := tx.QueryRow(ctx, query, scope, key).Scan(&id, &value, &valueType, &description)
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrConfigNotFound
		}
		if err != nil {
			return fmt.Errorf("delete config error: %w", err)
		}

		old := models.Config{ID: id, Scope: scope, Key: key, Value: value, ValueType: valueType, Description: description}
		return r.insertHistory(ctx, tx, actor, "config", id, "delete", configHistoryValue(old), nil)
	})
}

func (r *Repository) ListFlags(ctx context.Context, scope, search string) ([]models.Flag, error) {
	query, args := buildListQuery(`
		SELECT id, scope, key, description, rollout_percent, updated_by, created_at, updated_at
		FROM oc_flag
	`, scope, search)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list flags error: %w", err)
	}
	defer rows.Close()

	flags := make([]models.Flag, 0)
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var f models.Flag
		if err := rows.Scan(&f.ID, &f.Scope, &f.Key, &f.Description, &f.RolloutPercent, &f.UpdatedBy, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan flag: %w", err)
		}
		flags = append(flags, f)
		ids = append(ids, f.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list flags rows: %w", err)
	}

	if err := r.attachForces(ctx, r.pool, flags, ids); err != nil {
		return nil, err
	}

	return flags, nil
}

func (r *Repository) GetFlag(ctx context.Context, scope, key string) (*models.Flag, error) {
	const query = `
		SELECT id, scope, key, description, rollout_percent, updated_by, created_at, updated_at
		FROM oc_flag
		WHERE scope = $1 AND key = $2
	`

	return r.queryOneFlag(ctx, r.pool, query, scope, key)
}

func (r *Repository) CreateFlag(ctx context.Context, actor uuid.UUID, flag models.Flag) (*models.Flag, error) {
	var created models.Flag

	err := r.withMutation(ctx, func(tx pgx.Tx) error {
		const query = `
			INSERT INTO oc_flag (scope, key, description, rollout_percent, updated_by)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at, updated_at
		`

		r.logger.DebugContext(ctx, "executing query",
			slog.String("op", "create flag"),
			slog.String("scope", flag.Scope),
			slog.String("key", flag.Key),
		)

		err := tx.QueryRow(ctx, query, flag.Scope, flag.Key, flag.Description, flag.RolloutPercent, actor).
			Scan(&created.ID, &created.CreatedAt, &created.UpdatedAt)
		if err != nil {
			parsed := postgres.ParseError(err)
			switch {
			case errors.Is(parsed, postgres.ErrUniqViolation):
				return models.ErrFlagExists
			case errors.Is(parsed, postgres.ErrFKViolation):
				return models.ErrScopeNotFound
			}
			return fmt.Errorf("create flag error: %w", err)
		}

		created.Scope, created.Key = flag.Scope, flag.Key
		created.Description, created.RolloutPercent = flag.Description, flag.RolloutPercent
		created.UpdatedBy = &actor

		return r.insertHistory(ctx, tx, actor, "flag", created.ID, "create", nil, flagHistoryValue(flag))
	})
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *Repository) UpdateRollout(ctx context.Context, actor uuid.UUID, scope, key string, percent int16) (*models.Flag, error) {
	var updated models.Flag

	err := r.withMutation(ctx, func(tx pgx.Tx) error {
		const lockQuery = `
			SELECT id, scope, key, description, rollout_percent, updated_by, created_at, updated_at
			FROM oc_flag
			WHERE scope = $1 AND key = $2
			FOR UPDATE
		`

		old, err := r.queryOneFlag(ctx, tx, lockQuery, scope, key)
		if err != nil {
			return err
		}

		const query = `
			UPDATE oc_flag
			SET rollout_percent = $2, updated_by = $3
			WHERE id = $1
			RETURNING updated_at
		`

		r.logger.DebugContext(ctx, "executing query",
			slog.String("op", "update rollout"),
			slog.String("scope", scope),
			slog.String("key", key),
			slog.Int("percent", int(percent)),
		)

		if err := tx.QueryRow(ctx, query, old.ID, percent, actor).Scan(&updated.UpdatedAt); err != nil {
			return fmt.Errorf("update rollout error: %w", err)
		}

		updated.ID, updated.Scope, updated.Key = old.ID, old.Scope, old.Key
		updated.Description, updated.CreatedAt = old.Description, old.CreatedAt
		updated.RolloutPercent = percent
		updated.UpdatedBy = &actor

		// В историю пишем только изменение раскатки — компактный диф.
		return r.insertHistory(ctx, tx, actor, "flag", old.ID, "update",
			map[string]any{"rollout_percent": old.RolloutPercent},
			map[string]any{"rollout_percent": percent},
		)
	})
	if err != nil {
		return nil, err
	}

	return &updated, nil
}

func (r *Repository) SetForce(ctx context.Context, actor uuid.UUID, scope, key string, userID uuid.UUID, force string) error {
	return r.withMutation(ctx, func(tx pgx.Tx) error {
		flag, err := r.queryOneFlag(ctx, tx, `
			SELECT id, scope, key, description, rollout_percent, updated_by, created_at, updated_at
			FROM oc_flag
			WHERE scope = $1 AND key = $2
			FOR UPDATE
		`, scope, key)
		if err != nil {
			return err
		}

		const query = `
			INSERT INTO oc_flag_user (flag_id, user_id, force, created_by)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (flag_id, user_id) DO UPDATE
				SET force = EXCLUDED.force
		`

		r.logger.DebugContext(ctx, "executing query",
			slog.String("op", "set force"),
			slog.String("scope", scope),
			slog.String("key", key),
			slog.String("userID", userID.String()),
			slog.String("force", force),
		)

		if _, err := tx.Exec(ctx, query, flag.ID, userID, force, actor); err != nil {
			return fmt.Errorf("set force error: %w", err)
		}

		action := "force_on"
		if force == "off" {
			action = "force_off"
		}

		return r.insertHistory(ctx, tx, actor, "flag_user", flag.ID, action, nil,
			map[string]any{"user_id": userID, "force": force},
		)
	})
}

func (r *Repository) RemoveForce(ctx context.Context, actor uuid.UUID, scope, key string, userID uuid.UUID) error {
	return r.withMutation(ctx, func(tx pgx.Tx) error {
		flag, err := r.queryOneFlag(ctx, tx, `
			SELECT id, scope, key, description, rollout_percent, updated_by, created_at, updated_at
			FROM oc_flag
			WHERE scope = $1 AND key = $2
			FOR UPDATE
		`, scope, key)
		if err != nil {
			return err
		}

		const query = `
			DELETE FROM oc_flag_user
			WHERE flag_id = $1 AND user_id = $2
			RETURNING force
		`

		r.logger.DebugContext(ctx, "executing query",
			slog.String("op", "remove force"),
			slog.String("scope", scope),
			slog.String("key", key),
			slog.String("userID", userID.String()),
		)

		var force string
		err = tx.QueryRow(ctx, query, flag.ID, userID).Scan(&force)
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrFlagNotFound
		}
		if err != nil {
			return fmt.Errorf("remove force error: %w", err)
		}

		return r.insertHistory(ctx, tx, actor, "flag_user", flag.ID, "delete",
			map[string]any{"user_id": userID, "force": force},
			nil,
		)
	})
}

func (r *Repository) DeleteFlag(ctx context.Context, actor uuid.UUID, scope, key string) error {
	return r.withMutation(ctx, func(tx pgx.Tx) error {
		const query = `
			DELETE FROM oc_flag
			WHERE scope = $1 AND key = $2
			RETURNING id, description, rollout_percent
		`

		r.logger.DebugContext(ctx, "executing query",
			slog.String("op", "delete flag"),
			slog.String("scope", scope),
			slog.String("key", key),
		)

		var (
			id          uuid.UUID
			description string
			rollout     int16
		)
		err := tx.QueryRow(ctx, query, scope, key).Scan(&id, &description, &rollout)
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrFlagNotFound
		}
		if err != nil {
			return fmt.Errorf("delete flag error: %w", err)
		}

		old := models.Flag{ID: id, Scope: scope, Key: key, Description: description, RolloutPercent: rollout}
		return r.insertHistory(ctx, tx, actor, "flag", id, "delete", flagHistoryValue(old), nil)
	})
}

// ListHistory отдаёт журнал новее курсора (id DESC). Append-only + монотонный
// BIGSERIAL делают курсор стабильным без тай-брейкеров.
func (r *Repository) ListHistory(ctx context.Context, filter models.HistoryFilter, cursorID *int64, limit int) ([]models.HistoryEntry, *int64, error) {
	conditions := []string{"TRUE"}
	args := make([]any, 0, 4)
	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.EntityType != "" {
		conditions = append(conditions, "h.entity_type = "+next(filter.EntityType))
	}

	if filter.Scope != "" || filter.Key != "" {
		table, ok := map[string]string{
			"config": "oc_config",
			"flag":   "oc_flag",
		}[filter.EntityType]
		if !ok {
			return nil, nil, nil
		}

		conditions = append(conditions, fmt.Sprintf(
			"h.entity_id IN (SELECT id FROM %s WHERE scope = %s AND key = %s)",
			table, next(filter.Scope), next(filter.Key),
		))
	}

	if cursorID != nil {
		conditions = append(conditions, "h.id < "+next(*cursorID))
	}

	query := fmt.Sprintf(`
		SELECT h.id, h.actor_id, h.entity_type, h.entity_id, h.action, h.old_value::text, h.new_value::text, h.created_at
		FROM oc_history h
		WHERE %s
		ORDER BY h.id DESC
		LIMIT %d
	`, strings.Join(conditions, " AND "), limit+1)

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "list history"),
		slog.String("entity_type", filter.EntityType),
		slog.String("scope", filter.Scope),
		slog.String("key", filter.Key),
	)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list history error: %w", err)
	}
	defer rows.Close()

	entries := make([]models.HistoryEntry, 0, limit)
	for rows.Next() {
		var e models.HistoryEntry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.EntityType, &e.EntityID, &e.Action, &e.OldValue, &e.NewValue, &e.CreatedAt); err != nil {
			return nil, nil, fmt.Errorf("scan history entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("list history rows: %w", err)
	}

	var nextCursor *int64
	if len(entries) > limit {
		last := entries[limit-1].ID
		nextCursor = &last
		entries = entries[:limit]
	}

	return entries, nextCursor, nil
}

// withMutation выполняет fn в транзакции, сериализованной advisory-блокировкой
// (идиома refund/chargeback из 000004). Блокировка снимается по COMMIT/ROLLBACK.
func (r *Repository) withMutation(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin mutation tx: %w", err)
	}

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, mutationLockKey); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("acquire mutation lock: %w", err)
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) insertHistory(ctx context.Context, tx pgx.Tx, actor uuid.UUID, entityType string, entityID uuid.UUID, action string, oldValue, newValue any) error {
	oldJSON, err := historyValue(oldValue)
	if err != nil {
		return err
	}
	newJSON, err := historyValue(newValue)
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO oc_history (actor_id, entity_type, entity_id, action, old_value, new_value)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb)
	`

	if _, err := tx.Exec(ctx, query, actor, entityType, entityID, action, oldJSON, newJSON); err != nil {
		return fmt.Errorf("insert history error: %w", err)
	}

	return nil
}

// configHistoryValue и flagHistoryValue дают компактный JSON для журнала:
// без служебных полей (id, временные метки), которые и так есть в записи истории.
func configHistoryValue(cfg models.Config) map[string]any {
	return map[string]any{
		"scope":       cfg.Scope,
		"key":         cfg.Key,
		"value":       cfg.Value,
		"value_type":  cfg.ValueType,
		"description": cfg.Description,
	}
}

func flagHistoryValue(flag models.Flag) map[string]any {
	return map[string]any{
		"scope":           flag.Scope,
		"key":             flag.Key,
		"description":     flag.Description,
		"rollout_percent": flag.RolloutPercent,
	}
}

func historyValue(v any) (any, error) {
	if v == nil {
		return nil, nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal history value: %w", err)
	}

	return string(data), nil
}

func (r *Repository) queryConfigs(ctx context.Context, q querier, scopes []string) ([]models.Config, error) {
	const query = `
		SELECT id, scope, key, value::text, value_type, description, updated_by, created_at, updated_at
		FROM oc_config
		WHERE scope = ANY($1)
		ORDER BY scope, key
	`

	rows, err := q.Query(ctx, query, scopes)
	if err != nil {
		return nil, fmt.Errorf("query configs error: %w", err)
	}
	defer rows.Close()

	configs := make([]models.Config, 0)
	for rows.Next() {
		c, err := scanConfigRow(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query configs rows: %w", err)
	}

	return configs, nil
}

func (r *Repository) queryFlags(ctx context.Context, q querier, scopes []string) ([]models.Flag, error) {
	const query = `
		SELECT id, scope, key, description, rollout_percent, updated_by, created_at, updated_at
		FROM oc_flag
		WHERE scope = ANY($1)
		ORDER BY scope, key
	`

	rows, err := q.Query(ctx, query, scopes)
	if err != nil {
		return nil, fmt.Errorf("query flags error: %w", err)
	}
	defer rows.Close()

	flags := make([]models.Flag, 0)
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var f models.Flag
		if err := rows.Scan(&f.ID, &f.Scope, &f.Key, &f.Description, &f.RolloutPercent, &f.UpdatedBy, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan flag: %w", err)
		}
		flags = append(flags, f)
		ids = append(ids, f.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query flags rows: %w", err)
	}

	if err := r.attachForces(ctx, q, flags, ids); err != nil {
		return nil, err
	}

	return flags, nil
}

func (r *Repository) queryOneConfig(ctx context.Context, q querier, query string, args ...any) (*models.Config, error) {
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query config error: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("query config rows: %w", err)
		}
		return nil, models.ErrConfigNotFound
	}

	cfg, err := scanConfigRow(rows)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (r *Repository) queryOneFlag(ctx context.Context, q querier, query string, args ...any) (*models.Flag, error) {
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query flag error: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("query flag rows: %w", err)
		}
		return nil, models.ErrFlagNotFound
	}

	var f models.Flag
	if err := rows.Scan(&f.ID, &f.Scope, &f.Key, &f.Description, &f.RolloutPercent, &f.UpdatedBy, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan flag: %w", err)
	}

	return &f, nil
}

func (r *Repository) attachForces(ctx context.Context, q querier, flags []models.Flag, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	const query = `
		SELECT flag_id, user_id, force
		FROM oc_flag_user
		WHERE flag_id = ANY($1)
	`

	rows, err := q.Query(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("query forces error: %w", err)
	}
	defer rows.Close()

	index := make(map[uuid.UUID]int, len(flags))
	for i := range flags {
		index[flags[i].ID] = i
		flags[i].ForceOn = make([]uuid.UUID, 0)
		flags[i].ForceOff = make([]uuid.UUID, 0)
	}

	for rows.Next() {
		var (
			flagID uuid.UUID
			userID uuid.UUID
			force  string
		)
		if err := rows.Scan(&flagID, &userID, &force); err != nil {
			return fmt.Errorf("scan force: %w", err)
		}

		if i, ok := index[flagID]; ok {
			switch force {
			case "on":
				flags[i].ForceOn = append(flags[i].ForceOn, userID)
			case "off":
				flags[i].ForceOff = append(flags[i].ForceOff, userID)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("query forces rows: %w", err)
	}

	return nil
}

func scanConfigRow(rows pgx.Rows) (*models.Config, error) {
	var c models.Config
	if err := rows.Scan(&c.ID, &c.Scope, &c.Key, &c.Value, &c.ValueType, &c.Description, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan config: %w", err)
	}
	return &c, nil
}

// buildListQuery добавляет фильтры скоупа и поиска по названию (ILIKE, задание:
// «по тогглам и ключам должна быть возможность поиска по названию»).
func buildListQuery(base, scope, search string) (string, []any) {
	conditions := []string{"TRUE"}
	args := make([]any, 0, 2)

	if scope != "" {
		args = append(args, scope)
		conditions = append(conditions, fmt.Sprintf("scope = $%d", len(args)))
	}
	if search != "" {
		args = append(args, "%"+escapeLike(search)+"%")
		conditions = append(conditions, fmt.Sprintf(`"key" ILIKE $%d`, len(args)))
	}

	return fmt.Sprintf("%s WHERE %s ORDER BY scope, \"key\"", base, strings.Join(conditions, " AND ")), args
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

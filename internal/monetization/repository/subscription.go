package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	billingPlanIntervalUnitDay = "day"
	billingPlanIntervalCount30 = 30
	subscriptionTierCurrency   = "RUB"
)

func (r *MonetizationRepository) CreateSubscriptionTier(
	ctx context.Context,
	authorID uuid.UUID,
	req dto.CreateSubscriptionTierRequest,
) (*models.SubscriptionTier, error) {
	if authorID == uuid.Nil || req.Name == "" || req.Description == "" || req.PriceMinor <= 0 {
		return nil, models.ErrSubscriptionTierValidation
	}

	tier := &models.SubscriptionTier{
		AuthorID:    authorID,
		Name:        req.Name,
		Description: req.Description,
		PriceMinor:  req.PriceMinor,
		Currency:    subscriptionTierCurrency,
	}

	r.logger.DebugContext(ctx, "executing subscription tier transaction",
		slog.String("op", "create subscription tier"),
		slog.String("author_id", authorID.String()),
	)

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const checkActivePrice = `
			SELECT EXISTS (
				SELECT 1
				FROM billing_plan bp
				JOIN subscription_type st ON st.id = bp.subscription_type_id
				WHERE st.author_id = $1
					AND st.retired_at IS NULL
					AND bp.retired_at IS NULL
					AND bp.interval_unit = $2
					AND bp.interval_count = $3
					AND bp.currency = $4
					AND bp.amount_minor = $5
			)
		`
		var activePriceExists bool
		if err := tx.QueryRow(ctx, checkActivePrice,
			authorID,
			billingPlanIntervalUnitDay,
			billingPlanIntervalCount30,
			subscriptionTierCurrency,
			req.PriceMinor,
		).Scan(&activePriceExists); err != nil {
			return fmt.Errorf("check active subscription tier price: %w", err)
		}
		if activePriceExists {
			return models.ErrSubscriptionTierConflict
		}

		const insertTier = `
			INSERT INTO subscription_type (author_id, name, description)
			VALUES ($1, $2, $3)
			RETURNING id, created_at, updated_at
		`
		if err := tx.QueryRow(ctx, insertTier, authorID, req.Name, req.Description).Scan(
			&tier.ID,
			&tier.CreatedAt,
			&tier.UpdatedAt,
		); err != nil {
			return mapSubscriptionTierConstraintError(err, true)
		}

		const insertPlan = `
			INSERT INTO billing_plan (
				subscription_type_id, interval_unit, interval_count, amount_minor, currency
			)
			VALUES ($1, $2, $3, $4, $5)
		`
		if _, err := tx.Exec(ctx, insertPlan,
			tier.ID,
			billingPlanIntervalUnitDay,
			billingPlanIntervalCount30,
			req.PriceMinor,
			subscriptionTierCurrency,
		); err != nil {
			return mapSubscriptionTierConstraintError(err, false)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create subscription tier transaction: %w", err)
	}

	return tier, nil
}

func (r *MonetizationRepository) UpdateSubscriptionTier(
	ctx context.Context,
	authorID, tierID uuid.UUID,
	req dto.UpdateSubscriptionTierRequest,
) (*models.SubscriptionTier, error) {
	if authorID == uuid.Nil || tierID == uuid.Nil || req.Name == "" || req.Description == "" ||
		req.PriceMinor <= 0 || (req.Status != "open" && req.Status != "archived") {
		return nil, models.ErrSubscriptionTierValidation
	}

	tier := &models.SubscriptionTier{}
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const selectTier = `
			SELECT id, author_id, name, description, retired_at, created_at, updated_at
			FROM subscription_type
			WHERE id = $1 AND author_id = $2
			FOR UPDATE
		`
		if err := tx.QueryRow(ctx, selectTier, tierID, authorID).Scan(
			&tier.ID,
			&tier.AuthorID,
			&tier.Name,
			&tier.Description,
			&tier.ArchivedAt,
			&tier.CreatedAt,
			&tier.UpdatedAt,
		); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.ErrSubscriptionTierNotFound
			}
			return fmt.Errorf("get subscription tier for update: %w", err)
		}
		if tier.ArchivedAt != nil {
			return models.ErrSubscriptionTierConflict
		}

		const selectPlan = `
			SELECT id, amount_minor, currency
			FROM billing_plan
			WHERE subscription_type_id = $1
				AND interval_unit = $2
				AND interval_count = $3
				AND currency = $4
				AND retired_at IS NULL
			ORDER BY created_at DESC, id DESC
			LIMIT 1
			FOR UPDATE
		`
		var planID uuid.UUID
		var previousPrice int64
		if err := tx.QueryRow(ctx, selectPlan,
			tierID,
			billingPlanIntervalUnitDay,
			billingPlanIntervalCount30,
			subscriptionTierCurrency,
		).Scan(&planID, &previousPrice, &tier.Currency); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("subscription tier %s has no billing plan: %w", tierID, models.ErrSubscriptionTierValidation)
			}
			return fmt.Errorf("get subscription tier billing plan: %w", err)
		}

		if req.Status == "open" {
			const checkPriceOrder = `
				SELECT EXISTS (
					SELECT 1
					FROM billing_plan bp
					JOIN subscription_type st ON st.id = bp.subscription_type_id
					WHERE st.author_id = $1
						AND st.id <> $2
						AND st.retired_at IS NULL
						AND bp.retired_at IS NULL
						AND bp.interval_unit = $3
						AND bp.interval_count = $4
						AND bp.currency = $5
						AND (
							bp.amount_minor = $6
							OR ($7::bigint < bp.amount_minor AND $6::bigint >= bp.amount_minor)
							OR ($7::bigint > bp.amount_minor AND $6::bigint <= bp.amount_minor)
						)
				)
			`
			var changesOrder bool
			if err := tx.QueryRow(ctx, checkPriceOrder,
				authorID,
				tierID,
				billingPlanIntervalUnitDay,
				billingPlanIntervalCount30,
				subscriptionTierCurrency,
				req.PriceMinor,
				previousPrice,
			).Scan(&changesOrder); err != nil {
				return fmt.Errorf("check subscription tier price order: %w", err)
			}
			if changesOrder {
				return models.ErrSubscriptionTierConflict
			}
		}

		if req.PriceMinor != previousPrice {
			const updatePlan = `
				UPDATE billing_plan
				SET amount_minor = $1
				WHERE id = $2
			`
			if _, err := tx.Exec(ctx, updatePlan, req.PriceMinor, planID); err != nil {
				return mapSubscriptionTierConstraintError(err, false)
			}
		}

		const updateTier = `
			UPDATE subscription_type
			SET name = $3,
				description = $4,
				retired_at = CASE
					WHEN $5 = 'archived' THEN COALESCE(retired_at, now())
					ELSE NULL
				END
			WHERE id = $1 AND author_id = $2
			RETURNING retired_at, updated_at
		`
		if err := tx.QueryRow(ctx, updateTier, tierID, authorID, req.Name, req.Description, req.Status).
			Scan(&tier.ArchivedAt, &tier.UpdatedAt); err != nil {
			return mapSubscriptionTierConstraintError(err, false)
		}

		tier.Name = req.Name
		tier.Description = req.Description
		tier.PriceMinor = req.PriceMinor
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update subscription tier transaction: %w", err)
	}

	return tier, nil
}

func mapSubscriptionTierConstraintError(err error, authorReference bool) error {
	parsedErr := postgres.ParseError(err)
	switch {
	case errors.Is(parsedErr, postgres.ErrUniqViolation):
		return models.ErrSubscriptionTierConflict
	case errors.Is(parsedErr, postgres.ErrCheckViolation):
		return models.ErrSubscriptionTierValidation
	case authorReference && errors.Is(parsedErr, postgres.ErrFKViolation):
		return models.ErrAuthorNotFound
	default:
		return err
	}
}

func (r *MonetizationRepository) GetSubscriptionTiers(ctx context.Context, authorID uuid.UUID, cursor *dto.SubscriptionTierCursor, limit int) ([]*models.SubscriptionTier, error) {
	const query = `
		SELECT
			st.id,
			st.author_id,
			st.name,
			st.description,
			bp.amount_minor,
			bp.currency,
			st.retired_at,
			st.created_at,
			st.updated_at
		FROM subscription_type st
		JOIN LATERAL (
			SELECT amount_minor, currency
			FROM billing_plan
			WHERE subscription_type_id = st.id
				AND interval_unit = 'day'
				AND interval_count = 30
				AND currency = 'RUB'
			ORDER BY retired_at NULLS FIRST, created_at DESC, id DESC
			LIMIT 1
		) bp ON TRUE
		WHERE st.author_id = $1
			AND (
				$2::bigint IS NULL
				OR (bp.amount_minor, st.id) > ($2::bigint, $3::uuid)
			)
		ORDER BY bp.amount_minor, st.id
		LIMIT $4
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get subscription tiers"),
		slog.String("author_id", authorID.String()),
		slog.Int("limit", limit),
		slog.Bool("has_cursor", cursor != nil),
	)

	price, id := subscriptionTierCursorArgs(cursor)
	rows, err := r.pool.Query(ctx, query, authorID, price, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get subscription tiers: %w", err)
	}

	return scanSubscriptionTiers(rows)
}

func (r *MonetizationRepository) GetSubscriptionOptions(
	ctx context.Context,
	readerID, authorID uuid.UUID,
	postID *uuid.UUID,
	cursor *dto.SubscriptionTierCursor,
	limit int,
) ([]*models.SubscriptionTier, *models.CurrentSubscription, error) {
	r.logger.DebugContext(ctx, "executing subscription options transaction",
		slog.String("op", "get subscription options"),
		slog.String("author_id", authorID.String()),
		slog.String("reader_id", readerID.String()),
		slog.Int("limit", limit),
		slog.Bool("has_post_filter", postID != nil),
		slog.Bool("has_cursor", cursor != nil),
	)

	var tiers []*models.SubscriptionTier
	var current *models.CurrentSubscription
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		var postThreshold *int64
		postIsPublic := false
		if postID != nil {
			var requiredTierID *uuid.UUID
			accessStageExists := false
			const currentAccessStageQuery = `
				SELECT subscription_type_id
				FROM post_access_stage
				WHERE post_id = $1
					AND starts_at <= now()
				ORDER BY starts_at DESC
				LIMIT 1
			`
			err := tx.QueryRow(ctx, currentAccessStageQuery, *postID).Scan(&requiredTierID)
			if errors.Is(err, pgx.ErrNoRows) {
				const legacyPostAccessQuery = `
					SELECT subscription_type_id
					FROM post_access
					WHERE post_id = $1
				`
				var legacyTierID *uuid.UUID
				err = tx.QueryRow(ctx, legacyPostAccessQuery, *postID).Scan(&legacyTierID)
				if err == nil {
					requiredTierID = legacyTierID
					accessStageExists = true
				} else if !errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("get legacy post access tier: %w", err)
				}
			} else if err != nil {
				return fmt.Errorf("get current post access stage: %w", err)
			} else {
				accessStageExists = true
			}

			if accessStageExists && requiredTierID == nil {
				postIsPublic = true
			}
			if requiredTierID != nil {
				const requiredTierPriceQuery = `
					SELECT amount_minor
					FROM billing_plan
					WHERE subscription_type_id = $1
						AND interval_unit = $2
						AND interval_count = $3
						AND currency = $4
					ORDER BY retired_at NULLS FIRST, created_at DESC, id DESC
					LIMIT 1
				`
				var price int64
				if err := tx.QueryRow(ctx, requiredTierPriceQuery,
					*requiredTierID,
					billingPlanIntervalUnitDay,
					billingPlanIntervalCount30,
					subscriptionTierCurrency,
				).Scan(&price); err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						return fmt.Errorf("post access tier has no billing plan: %w", models.ErrSubscriptionTargetNotFound)
					}
					return fmt.Errorf("get post access tier price: %w", err)
				}
				postThreshold = &price
			}
		}

		if readerID != uuid.Nil {
			const currentSubscriptionQuery = `
				SELECT
					s.id,
					st.id,
					st.name,
					CASE WHEN st.retired_at IS NULL THEN 'open' ELSE 'archived' END,
					bp.amount_minor,
					bp.currency,
					spe.period_end
				FROM subscription_period_with_end spe
				JOIN subscription s ON s.id = spe.subscription_id
				JOIN billing_plan bp ON bp.id = spe.billing_plan_id
				JOIN subscription_type st ON st.id = bp.subscription_type_id
				WHERE s.user_id = $1
					AND st.author_id = $2
					AND spe.period_start <= now()
					AND spe.period_end > now()
					AND spe.revoked_at IS NULL
				ORDER BY spe.period_start DESC, spe.created_at DESC, s.id DESC
				LIMIT 1
			`
			current = &models.CurrentSubscription{}
			if err := tx.QueryRow(ctx, currentSubscriptionQuery, readerID, authorID).Scan(
				&current.ID,
				&current.TierID,
				&current.TierName,
				&current.TierStatus,
				&current.PriceMinor,
				&current.Currency,
				&current.PeriodEndsAt,
			); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					current = nil
				} else {
					return fmt.Errorf("get current reader subscription: %w", err)
				}
			}
		}

		if postIsPublic ||
			(postThreshold != nil && current != nil && current.PriceMinor >= *postThreshold) ||
			readerID == authorID {
			tiers = make([]*models.SubscriptionTier, 0)
			return nil
		}

		const optionsQuery = `
			SELECT
				st.id,
				st.author_id,
				st.name,
				st.description,
				bp.amount_minor,
				bp.currency,
				st.retired_at,
				st.created_at,
				st.updated_at
			FROM subscription_type st
			JOIN LATERAL (
				SELECT amount_minor, currency
				FROM billing_plan
				WHERE subscription_type_id = st.id
					AND interval_unit = 'day'
					AND interval_count = 30
					AND currency = 'RUB'
					AND retired_at IS NULL
				ORDER BY created_at DESC, id DESC
				LIMIT 1
			) bp ON TRUE
			WHERE st.author_id = $1
				AND st.retired_at IS NULL
				AND ($2::bigint IS NULL OR bp.amount_minor >= $2)
				AND ($3::bigint IS NULL OR bp.amount_minor > $3)
				AND (
					$4::bigint IS NULL
					OR (bp.amount_minor, st.id) > ($4::bigint, $5::uuid)
				)
			ORDER BY bp.amount_minor, st.id
			LIMIT $6
		`
		var currentPrice *int64
		if current != nil {
			currentPrice = &current.PriceMinor
		}
		price, id := subscriptionTierCursorArgs(cursor)
		rows, err := tx.Query(ctx, optionsQuery,
			authorID,
			postThreshold,
			currentPrice,
			price,
			id,
			limit,
		)
		if err != nil {
			return fmt.Errorf("get subscription options: %w", err)
		}
		tiers, err = scanSubscriptionTiers(rows)
		if err != nil {
			return fmt.Errorf("scan subscription options: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("get subscription options transaction: %w", err)
	}

	return tiers, current, nil
}

func subscriptionTierCursorArgs(cursor *dto.SubscriptionTierCursor) (*int64, *uuid.UUID) {
	if cursor == nil {
		return nil, nil
	}

	return &cursor.PriceMinor, &cursor.TierID
}

func scanSubscriptionTiers(rows pgx.Rows) ([]*models.SubscriptionTier, error) {
	defer rows.Close()

	tiers := make([]*models.SubscriptionTier, 0)
	for rows.Next() {
		tier := &models.SubscriptionTier{}
		if err := rows.Scan(
			&tier.ID,
			&tier.AuthorID,
			&tier.Name,
			&tier.Description,
			&tier.PriceMinor,
			&tier.Currency,
			&tier.ArchivedAt,
			&tier.CreatedAt,
			&tier.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan subscription tier: %w", err)
		}
		tiers = append(tiers, tier)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscription tiers: %w", err)
	}

	return tiers, nil
}

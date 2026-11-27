package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type currentSubscriptionState struct {
	ID         uuid.UUID
	TierID     uuid.UUID
	PriceMinor int64
}

type subscriptionPurchaseState struct {
	current               *models.CurrentSubscription
	periodStart           time.Time
	authorID              uuid.UUID
	readerID              uuid.UUID
	action                string
	expectedCurrentTierID uuid.UUID
	postID                *uuid.UUID
}

func (r *MonetizationRepository) CreateSubscriptionWithPayment(
	ctx context.Context,
	req dto.CreateSubscriptionWithPayment,
) (*models.CurrentSubscription, *models.PaymentOrder, error) {
	r.logger.DebugContext(ctx, "executing subscription purchase transaction",
		slog.String("op", "create subscription with payment"),
		slog.String("reader_id", req.ReaderID.String()),
		slog.String("author_id", req.AuthorID.String()),
		slog.String("tier_id", req.TierID.String()),
		slog.String("action", req.Action),
	)

	var subscription *models.CurrentSubscription
	order := &models.PaymentOrder{}
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const insertOrder = `
			INSERT INTO payment_order (
				payer_id, request_key, purpose, origin, expected_amount_minor, currency, status, expires_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
			ON CONFLICT (payer_id, request_key) DO NOTHING
			RETURNING
				id, payer_id, request_key, purpose, origin, expected_amount_minor, currency, status,
				credited_attempt_id, expires_at, paid_at, canceled_at, created_at, updated_at
		`
		err := tx.QueryRow(ctx, insertOrder,
			req.ReaderID,
			req.RequestKey,
			models.PurposeSubscriptionPeriod,
			models.OriginManual,
			req.ExpectedPriceMinor,
			subscriptionTierCurrency,
			req.ExpiresAt,
		).Scan(
			&order.ID,
			&order.PayerID,
			&order.RequestKey,
			&order.Purpose,
			&order.Origin,
			&order.ExpectedAmountMinor,
			&order.Currency,
			&order.Status,
			&order.CreditedAttemptID,
			&order.ExpiresAt,
			&order.PaidAt,
			&order.CanceledAt,
			&order.CreatedAt,
			&order.UpdatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			order, subscription, err = getExistingSubscriptionPurchase(ctx, tx, req)
			return err
		}
		if err != nil {
			return fmt.Errorf("insert subscription payment order: %w", err)
		}

		archived, err := lockSubscriptionPurchaseTier(ctx, tx, req)
		if err != nil {
			return err
		}
		if archived {
			return models.ErrSubscriptionTierUnavailable
		}

		planID, price, err := lockSubscriptionPurchasePlan(ctx, tx, req)
		if err != nil {
			return err
		}
		if price != req.ExpectedPriceMinor {
			return models.ErrSubscriptionPriceChanged
		}

		current, err := lockCurrentSubscription(ctx, tx, req.ReaderID, req.AuthorID)
		if err != nil {
			return err
		}

		var subscriptionID uuid.UUID
		periodStartsRevoked := false
		if req.Action == "upgrade" {
			if current == nil ||
				req.ExpectedSubscriptionID == nil ||
				req.ExpectedCurrentTierID == nil ||
				current.ID != *req.ExpectedSubscriptionID ||
				current.TierID != *req.ExpectedCurrentTierID {
				return models.ErrSubscriptionChanged
			}
			if price <= current.PriceMinor {
				return models.ErrSubscriptionTierUnavailable
			}
			subscriptionID = current.ID
			periodStartsRevoked = true
		} else {
			if current != nil {
				return models.ErrSubscriptionChanged
			}

			existingID, existingStatus, err := getLatestSubscriptionForAuthor(ctx, tx, req.ReaderID, req.AuthorID)
			if err != nil {
				return err
			}
			if existingID == uuid.Nil {
				const insertSubscription = `
					INSERT INTO subscription (user_id, billing_plan_id, status)
					VALUES ($1, $2, 'pending')
					RETURNING id
				`
				if err := tx.QueryRow(ctx, insertSubscription, req.ReaderID, planID).Scan(&subscriptionID); err != nil {
					return mapSubscriptionPurchaseConstraintError(err)
				}
			} else {
				if existingStatus == "pending" || existingStatus == "paused" {
					return models.ErrSubscriptionChanged
				}
				const reuseSubscription = `
					UPDATE subscription
					SET billing_plan_id = $2,
						status = 'pending',
						cancellation_requested_at = NULL,
						ended_at = NULL
					WHERE id = $1
				`
				if _, err := tx.Exec(ctx, reuseSubscription, existingID, planID); err != nil {
					return mapSubscriptionPurchaseConstraintError(err)
				}
				subscriptionID = existingID
			}
		}

		const hasPendingPurchase = `
			SELECT EXISTS (
				SELECT 1
				FROM subscription_period sp
				JOIN payment_order po ON po.id = sp.payment_order_id
				WHERE sp.subscription_id = $1
					AND po.status IN ('pending', 'processing')
			)
		`
		var hasPending bool
		if err := tx.QueryRow(ctx, hasPendingPurchase, subscriptionID).Scan(&hasPending); err != nil {
			return fmt.Errorf("check pending subscription purchases: %w", err)
		}
		if hasPending {
			return models.ErrSubscriptionChanged
		}

		const insertPeriod = `
			INSERT INTO subscription_period (
				payment_order_id, subscription_id, billing_plan_id, period_start, revoked_at,
				request_action, expected_current_tier_id, post_id
			)
			VALUES ($1, $2, $3, now(), CASE WHEN $4 THEN now() ELSE NULL END, $5, $6, $7)
		`
		if _, err := tx.Exec(ctx, insertPeriod,
			order.ID,
			subscriptionID,
			planID,
			periodStartsRevoked,
			req.Action,
			req.ExpectedCurrentTierID,
			req.PostID,
		); err != nil {
			return mapSubscriptionPurchaseConstraintError(err)
		}
		if err := createInitialAttempt(ctx, tx, order.ID); err != nil {
			return err
		}

		subscription, err = getSubscriptionPurchaseByOrder(ctx, tx, order.ID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, nil, mapSubscriptionPurchaseTransactionError(err)
	}

	return subscription, order, nil
}

func lockSubscriptionPurchaseTier(
	ctx context.Context,
	tx pgx.Tx,
	req dto.CreateSubscriptionWithPayment,
) (bool, error) {
	const query = `
		SELECT retired_at
		FROM subscription_type
		WHERE id = $1 AND author_id = $2
		FOR UPDATE
	`
	var archivedAt *time.Time
	if err := tx.QueryRow(ctx, query, req.TierID, req.AuthorID).Scan(&archivedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, models.ErrSubscriptionTargetNotFound
		}
		return false, fmt.Errorf("lock subscription purchase tier: %w", err)
	}

	return archivedAt != nil, nil
}

func lockSubscriptionPurchasePlan(
	ctx context.Context,
	tx pgx.Tx,
	req dto.CreateSubscriptionWithPayment,
) (uuid.UUID, int64, error) {
	const query = `
		SELECT id, amount_minor
		FROM billing_plan
		WHERE subscription_type_id = $1
			AND interval_unit = $2
			AND interval_count = $3
			AND currency = $4
			AND retired_at IS NULL
		FOR UPDATE
	`
	var (
		planID uuid.UUID
		price  int64
	)
	if err := tx.QueryRow(ctx, query,
		req.TierID,
		billingPlanIntervalUnitDay,
		billingPlanIntervalCount30,
		subscriptionTierCurrency,
	).Scan(&planID, &price); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, 0, models.ErrSubscriptionTierUnavailable
		}
		return uuid.Nil, 0, fmt.Errorf("lock subscription purchase plan: %w", err)
	}

	return planID, price, nil
}

func lockCurrentSubscription(
	ctx context.Context,
	tx pgx.Tx,
	readerID, authorID uuid.UUID,
) (*currentSubscriptionState, error) {
	const query = `
		SELECT
			s.id,
			st.id,
			bp.amount_minor
		FROM subscription s
		JOIN subscription_period_with_end spe ON spe.subscription_id = s.id
		JOIN billing_plan bp ON bp.id = spe.billing_plan_id
		JOIN subscription_type st ON st.id = bp.subscription_type_id
		WHERE s.user_id = $1
			AND st.author_id = $2
			AND s.status = 'active'
			AND spe.period_start <= now()
			AND spe.period_end > now()
			AND spe.revoked_at IS NULL
		ORDER BY spe.period_start DESC, spe.created_at DESC, s.id DESC
		LIMIT 1
		FOR UPDATE OF s
	`
	current := &currentSubscriptionState{}
	if err := tx.QueryRow(ctx, query, readerID, authorID).Scan(
		&current.ID,
		&current.TierID,
		&current.PriceMinor,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lock current subscription: %w", err)
	}

	return current, nil
}

func getLatestSubscriptionForAuthor(
	ctx context.Context,
	tx pgx.Tx,
	readerID, authorID uuid.UUID,
) (uuid.UUID, string, error) {
	const query = `
		SELECT s.id, s.status
		FROM subscription s
		JOIN billing_plan bp ON bp.id = s.billing_plan_id
		JOIN subscription_type st ON st.id = bp.subscription_type_id
		WHERE s.user_id = $1 AND st.author_id = $2
		ORDER BY s.created_at DESC, s.id DESC
		LIMIT 1
		FOR UPDATE OF s
	`
	var id uuid.UUID
	var status string
	if err := tx.QueryRow(ctx, query, readerID, authorID).Scan(&id, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", nil
		}
		return uuid.Nil, "", fmt.Errorf("get existing subscription: %w", err)
	}

	return id, status, nil
}

func getExistingSubscriptionPurchase(
	ctx context.Context,
	tx pgx.Tx,
	req dto.CreateSubscriptionWithPayment,
) (*models.PaymentOrder, *models.CurrentSubscription, error) {
	const query = `
		SELECT
			id, payer_id, request_key, purpose, origin, expected_amount_minor, currency, status,
			credited_attempt_id, expires_at, paid_at, canceled_at, created_at, updated_at
		FROM payment_order
		WHERE payer_id = $1 AND request_key = $2
		FOR UPDATE
	`
	order := &models.PaymentOrder{}
	if err := tx.QueryRow(ctx, query, req.ReaderID, req.RequestKey).Scan(
		&order.ID,
		&order.PayerID,
		&order.RequestKey,
		&order.Purpose,
		&order.Origin,
		&order.ExpectedAmountMinor,
		&order.Currency,
		&order.Status,
		&order.CreditedAttemptID,
		&order.ExpiresAt,
		&order.PaidAt,
		&order.CanceledAt,
		&order.CreatedAt,
		&order.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, models.ErrPaymentOrderNotFound
		}
		return nil, nil, fmt.Errorf("get existing subscription payment order: %w", err)
	}

	if order.Purpose != models.PurposeSubscriptionPeriod ||
		order.Origin != models.OriginManual ||
		order.ExpectedAmountMinor != req.ExpectedPriceMinor ||
		order.Currency != subscriptionTierCurrency {
		return nil, nil, models.ErrIdempotencyKeyReused
	}

	purchase, err := getSubscriptionPurchaseState(ctx, tx, order.ID)
	if err != nil {
		return nil, nil, err
	}
	if purchase.readerID != req.ReaderID ||
		purchase.authorID != req.AuthorID ||
		purchase.current.TierID != req.TierID {
		return nil, nil, models.ErrIdempotencyKeyReused
	}

	if purchase.action != "" {
		if purchase.action != req.Action ||
			!equalOptionalUUID(purchase.expectedCurrentTierID, req.ExpectedCurrentTierID) ||
			!equalOptionalUUIDPtr(purchase.postID, req.PostID) {
			return nil, nil, models.ErrIdempotencyKeyReused
		}
		if req.Action == "upgrade" &&
			(req.ExpectedSubscriptionID == nil || *req.ExpectedSubscriptionID != purchase.current.ID) {
			return nil, nil, models.ErrIdempotencyKeyReused
		}
	} else {
		previousTierID, previousPeriodEnd, hasPrevious, err := getPreviousSubscriptionPeriod(
			ctx,
			tx,
			purchase.current.ID,
			purchase.periodStart,
		)
		if err != nil {
			return nil, nil, err
		}
		wasUpgrade := hasPrevious && previousPeriodEnd.After(purchase.periodStart)
		if req.Action == "upgrade" {
			if !wasUpgrade ||
				req.ExpectedSubscriptionID == nil ||
				req.ExpectedCurrentTierID == nil ||
				*req.ExpectedSubscriptionID != purchase.current.ID ||
				*req.ExpectedCurrentTierID != previousTierID {
				return nil, nil, models.ErrIdempotencyKeyReused
			}
		} else if wasUpgrade || req.ExpectedSubscriptionID != nil || req.ExpectedCurrentTierID != nil {
			return nil, nil, models.ErrIdempotencyKeyReused
		}
	}

	return order, purchase.current, nil
}

func getSubscriptionPurchaseByOrder(
	ctx context.Context,
	tx pgx.Tx,
	orderID uuid.UUID,
) (*models.CurrentSubscription, error) {
	purchase, err := getSubscriptionPurchaseState(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	return purchase.current, nil
}

func getSubscriptionPurchaseState(
	ctx context.Context,
	tx pgx.Tx,
	orderID uuid.UUID,
) (*subscriptionPurchaseState, error) {
	const query = `
		SELECT
			s.id,
			st.id,
			st.name,
			CASE WHEN st.retired_at IS NULL THEN 'open' ELSE 'archived' END,
			po.expected_amount_minor,
			po.currency,
			spe.period_start,
			spe.period_end,
			st.author_id,
			s.user_id,
			COALESCE(sp.request_action, ''),
			COALESCE(sp.expected_current_tier_id, '00000000-0000-0000-0000-000000000000'::uuid),
			sp.post_id IS NOT NULL,
			COALESCE(sp.post_id, '00000000-0000-0000-0000-000000000000'::uuid)
		FROM subscription_period_with_end spe
		JOIN payment_order po ON po.id = spe.payment_order_id
		JOIN subscription_period sp ON sp.payment_order_id = spe.payment_order_id
		JOIN subscription s ON s.id = spe.subscription_id
		JOIN billing_plan bp ON bp.id = spe.billing_plan_id
		JOIN subscription_type st ON st.id = bp.subscription_type_id
		WHERE spe.payment_order_id = $1
	`
	purchase := &subscriptionPurchaseState{current: &models.CurrentSubscription{}}
	var (
		hasPostID bool
		postID    uuid.UUID
	)
	if err := tx.QueryRow(ctx, query, orderID).Scan(
		&purchase.current.ID,
		&purchase.current.TierID,
		&purchase.current.TierName,
		&purchase.current.TierStatus,
		&purchase.current.PriceMinor,
		&purchase.current.Currency,
		&purchase.periodStart,
		&purchase.current.PeriodEndsAt,
		&purchase.authorID,
		&purchase.readerID,
		&purchase.action,
		&purchase.expectedCurrentTierID,
		&hasPostID,
		&postID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrSubscriptionTargetNotFound
		}
		return nil, fmt.Errorf("get subscription purchase result: %w", err)
	}
	if hasPostID {
		purchase.postID = &postID
	}

	return purchase, nil
}

func getPreviousSubscriptionPeriod(
	ctx context.Context,
	tx pgx.Tx,
	subscriptionID uuid.UUID,
	before time.Time,
) (uuid.UUID, time.Time, bool, error) {
	const query = `
		SELECT st.id, spe.period_end
		FROM subscription_period_with_end spe
		JOIN billing_plan bp ON bp.id = spe.billing_plan_id
		JOIN subscription_type st ON st.id = bp.subscription_type_id
		WHERE spe.subscription_id = $1 AND spe.period_start < $2
		ORDER BY spe.period_start DESC, spe.created_at DESC, spe.payment_order_id DESC
		LIMIT 1
	`
	var (
		tierID    uuid.UUID
		periodEnd time.Time
	)
	if err := tx.QueryRow(ctx, query, subscriptionID, before).Scan(&tierID, &periodEnd); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, time.Time{}, false, nil
		}
		return uuid.Nil, time.Time{}, false, fmt.Errorf("get previous subscription period: %w", err)
	}

	return tierID, periodEnd, true, nil
}

func equalOptionalUUID(stored uuid.UUID, requested *uuid.UUID) bool {
	if requested == nil {
		return stored == uuid.Nil
	}
	return stored == *requested
}

func equalOptionalUUIDPtr(stored, requested *uuid.UUID) bool {
	if stored == nil || requested == nil {
		return stored == nil && requested == nil
	}
	return *stored == *requested
}

func (r *MonetizationRepository) ActivateSubscription(
	ctx context.Context,
	paymentOrderID uuid.UUID,
) (*models.CurrentSubscription, error) {
	var current *models.CurrentSubscription
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const getPurchase = `
			SELECT
				po.status,
				sp.subscription_id,
				sp.billing_plan_id,
				sp.period_start,
				s.status,
				st.id,
				st.name,
				CASE WHEN st.retired_at IS NULL THEN 'open' ELSE 'archived' END,
				po.expected_amount_minor,
				po.currency,
				spe.period_end
			FROM payment_order po
			JOIN subscription_period sp ON sp.payment_order_id = po.id
			JOIN subscription_period_with_end spe ON spe.payment_order_id = po.id
			JOIN subscription s ON s.id = sp.subscription_id
			JOIN billing_plan bp ON bp.id = sp.billing_plan_id
			JOIN subscription_type st ON st.id = bp.subscription_type_id
			WHERE po.id = $1 AND po.purpose = 'subscription_period'
			FOR UPDATE OF po, s
		`
		var (
			orderStatus        models.PaymentOrderStatus
			subscriptionID     uuid.UUID
			planID             uuid.UUID
			periodStart        time.Time
			subscriptionStatus string
		)
		current = &models.CurrentSubscription{}
		if err := tx.QueryRow(ctx, getPurchase, paymentOrderID).Scan(
			&orderStatus,
			&subscriptionID,
			&planID,
			&periodStart,
			&subscriptionStatus,
			&current.TierID,
			&current.TierName,
			&current.TierStatus,
			&current.PriceMinor,
			&current.Currency,
			&current.PeriodEndsAt,
		); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.ErrPaymentOrderNotFound
			}
			return fmt.Errorf("get subscription payment result: %w", err)
		}
		current.ID = subscriptionID
		if orderStatus != models.StatusPaid {
			return models.ErrPaymentOrderValidation
		}

		const hasLaterPeriod = `
			SELECT EXISTS (
				SELECT 1
				FROM subscription_period
				WHERE subscription_id = $1 AND period_start > $2
			)
		`
		var superseded bool
		if err := tx.QueryRow(ctx, hasLaterPeriod, subscriptionID, periodStart).Scan(&superseded); err != nil {
			return fmt.Errorf("check later subscription periods: %w", err)
		}
		if superseded {
			return nil
		}
		if subscriptionStatus != "pending" && subscriptionStatus != "active" {
			return models.ErrSubscriptionChanged
		}

		const revokePreviousPeriods = `
			UPDATE subscription_period
			SET revoked_at = now()
			WHERE subscription_id = $1
				AND payment_order_id <> $2
				AND revoked_at IS NULL
		`
		if _, err := tx.Exec(ctx, revokePreviousPeriods, subscriptionID, paymentOrderID); err != nil {
			return fmt.Errorf("end previous subscription periods: %w", err)
		}

		const activatePeriod = `
			UPDATE subscription_period
			SET revoked_at = NULL
			WHERE payment_order_id = $1
		`
		if _, err := tx.Exec(ctx, activatePeriod, paymentOrderID); err != nil {
			return fmt.Errorf("activate subscription period: %w", err)
		}

		const activateSubscription = `
			UPDATE subscription
			SET billing_plan_id = $2,
				status = 'active',
				cancellation_requested_at = NULL,
				ended_at = NULL
			WHERE id = $1 AND status IN ('pending', 'active')
		`
		result, err := tx.Exec(ctx, activateSubscription, subscriptionID, planID)
		if err != nil {
			return mapSubscriptionPurchaseConstraintError(err)
		}
		if result.RowsAffected() != 1 {
			return models.ErrSubscriptionChanged
		}

		return nil
	})
	if err != nil {
		return nil, mapSubscriptionPurchaseTransactionError(fmt.Errorf("activate subscription transaction: %w", err))
	}

	return current, nil
}

func (r *MonetizationRepository) FailSubscriptionPurchase(ctx context.Context, paymentOrderID uuid.UUID) error {
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const getSubscriptionID = `
			SELECT s.id, s.status
			FROM subscription_period sp
			JOIN subscription s ON s.id = sp.subscription_id
			WHERE sp.payment_order_id = $1
			FOR UPDATE OF s
		`
		var (
			subscriptionID uuid.UUID
			status         string
		)
		if err := tx.QueryRow(ctx, getSubscriptionID, paymentOrderID).Scan(&subscriptionID, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.ErrSubscriptionTargetNotFound
			}
			return fmt.Errorf("get failed subscription purchase: %w", err)
		}

		const revokeFailedPeriod = `
			UPDATE subscription_period
			SET revoked_at = COALESCE(revoked_at, now())
			WHERE payment_order_id = $1
		`
		if _, err := tx.Exec(ctx, revokeFailedPeriod, paymentOrderID); err != nil {
			return fmt.Errorf("revoke failed subscription period: %w", err)
		}

		if status == "pending" {
			const cancelPendingSubscription = `
				UPDATE subscription
				SET status = 'canceled', ended_at = now(), cancellation_requested_at = NULL
				WHERE id = $1 AND status = 'pending'
			`
			if _, err := tx.Exec(ctx, cancelPendingSubscription, subscriptionID); err != nil {
				return mapSubscriptionPurchaseConstraintError(err)
			}
		}

		return nil
	})
	if err != nil {
		return mapSubscriptionPurchaseTransactionError(fmt.Errorf("fail subscription purchase transaction: %w", err))
	}

	return nil
}

func (r *MonetizationRepository) CancelSubscription(ctx context.Context, subscriptionID uuid.UUID) error {
	if subscriptionID == uuid.Nil {
		return models.ErrSubscriptionTargetNotFound
	}

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const lockSubscription = `SELECT status FROM subscription WHERE id = $1 FOR UPDATE`
		var status string
		if err := tx.QueryRow(ctx, lockSubscription, subscriptionID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.ErrSubscriptionTargetNotFound
			}
			return fmt.Errorf("lock subscription to cancel: %w", err)
		}

		if status != "canceled" && status != "expired" {
			const cancelSubscription = `
				UPDATE subscription
				SET status = 'canceled', ended_at = now(), cancellation_requested_at = NULL
				WHERE id = $1
			`
			if _, err := tx.Exec(ctx, cancelSubscription, subscriptionID); err != nil {
				return mapSubscriptionPurchaseConstraintError(err)
			}
		}

		const revokePeriods = `
			UPDATE subscription_period
			SET revoked_at = COALESCE(revoked_at, now())
			WHERE subscription_id = $1
		`
		if _, err := tx.Exec(ctx, revokePeriods, subscriptionID); err != nil {
			return fmt.Errorf("revoke canceled subscription periods: %w", err)
		}

		return nil
	})
	if err != nil {
		return mapSubscriptionPurchaseTransactionError(fmt.Errorf("cancel subscription transaction: %w", err))
	}

	return nil
}

func mapSubscriptionPurchaseConstraintError(err error) error {
	parsedErr := postgres.ParseError(err)
	switch {
	case errors.Is(parsedErr, postgres.ErrUniqViolation):
		return models.ErrSubscriptionChanged
	case errors.Is(parsedErr, postgres.ErrFKViolation):
		return models.ErrSubscriptionTargetNotFound
	case errors.Is(parsedErr, postgres.ErrCheckViolation):
		return models.ErrSubscriptionTierValidation
	default:
		return err
	}
}

func mapSubscriptionPurchaseTransactionError(err error) error {
	switch {
	case errors.Is(err, models.ErrSubscriptionTierValidation),
		errors.Is(err, models.ErrSubscriptionTierUnavailable),
		errors.Is(err, models.ErrSubscriptionPriceChanged),
		errors.Is(err, models.ErrSubscriptionChanged),
		errors.Is(err, models.ErrSubscriptionTargetNotFound),
		errors.Is(err, models.ErrIdempotencyKeyReused),
		errors.Is(err, models.ErrPaymentOrderNotFound),
		errors.Is(err, models.ErrPaymentOrderValidation),
		errors.Is(err, models.ErrPaymentAttemptValidation):
		return err
	default:
		return mapSubscriptionPurchaseConstraintError(err)
	}
}

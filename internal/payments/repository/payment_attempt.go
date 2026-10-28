package paymentrepository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PaymentAttemptRepository struct {
	pool PgxCliPool
}

func NewPaymentAttemptRepository(pool PgxCliPool) *PaymentAttemptRepository {
	return &PaymentAttemptRepository{pool: pool}
}

const paymentAttemptColumns = `id, payment_order_id, provider, external_transaction_id, status, charged_amount_minor, charged_currency, failure_reason, completed_at, created_at, updated_at`

func (r *PaymentAttemptRepository) Create(ctx context.Context, attempt *models.PaymentAttempt) (*models.PaymentAttempt, error) {
	const query = `
		INSERT INTO payment_attempt (payment_order_id, provider, status)
		SELECT po.id, $2, 'created'
		FROM payment_order po
		WHERE po.id = $1 AND po.status IN ('pending', 'processing')
		FOR SHARE OF po
		RETURNING id, status, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query, attempt.PaymentOrderID, attempt.Provider).Scan(
		&attempt.ID,
		&attempt.Status,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrPaymentOrderNotFound
		}
		parsed := postgres.ParseError(err)
		if errors.Is(parsed, postgres.ErrFKViolation) ||
			errors.Is(parsed, postgres.ErrCheckViolation) ||
			errors.Is(parsed, postgres.ErrUniqViolation) {
			return nil, models.ErrPaymentAttemptValidation
		}

		return nil, fmt.Errorf("create payment attempt: %w", err)
	}

	return attempt, nil
}

func (r *PaymentAttemptRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PaymentAttempt, error) {
	const query = `SELECT ` + paymentAttemptColumns + ` FROM payment_attempt WHERE id = $1`

	attempt := &models.PaymentAttempt{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&attempt.ID,
		&attempt.PaymentOrderID,
		&attempt.Provider,
		&attempt.ExternalTransactionID,
		&attempt.Status,
		&attempt.ChargedAmountMinor,
		&attempt.ChargedCurrency,
		&attempt.FailureReason,
		&attempt.CompletedAt,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrPaymentAttemptNotFound
		}
		return nil, fmt.Errorf("get payment attempt error: %w", err)
	}
	return attempt, nil
}

func (r *PaymentAttemptRepository) MarkProcessing(ctx context.Context, id uuid.UUID) error {
	return r.changeStatus(ctx, id, models.AttemptStatusProcessing, "")
}

func (r *PaymentAttemptRepository) MarkFailed(ctx context.Context, id uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return models.ErrPaymentAttemptValidation
	}

	return r.changeStatus(ctx, id, models.AttemptStatusFailed, reason)
}

func (r *PaymentAttemptRepository) MarkCanceled(ctx context.Context, id uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return models.ErrPaymentAttemptValidation
	}

	return r.changeStatus(ctx, id, models.AttemptStatusCanceled, reason)
}

func (r *PaymentAttemptRepository) changeStatus(ctx context.Context, id uuid.UUID, targetStatus models.PaymentAttemptStatus, reason string) error {
	var (
		query string
		args  []any
	)

	switch targetStatus {
	case models.AttemptStatusProcessing:
		query = `
			UPDATE payment_attempt
			SET status = 'processing', failure_reason = NULL
			WHERE id = $1 AND status = 'created'
		`
		args = []any{id}
	case models.AttemptStatusFailed:
		query = `
			UPDATE payment_attempt
			SET status = 'failed', failure_reason = $2
			WHERE id = $1 AND status IN ('created', 'processing')
		`
		args = []any{id, reason}
	case models.AttemptStatusCanceled:
		query = `
			UPDATE payment_attempt
			SET status = 'canceled', failure_reason = $2
			WHERE id = $1 AND status IN ('created', 'processing')
		`
		args = []any{id, reason}
	default:
		return models.ErrPaymentAttemptValidation
	}

	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("change payment attempt status: %w", err)
	}

	if result.RowsAffected() != 1 {
		var (
			currentStatus models.PaymentAttemptStatus
			currentReason *string
		)

		err = r.pool.QueryRow(ctx, `SELECT status, failure_reason FROM payment_attempt WHERE id = $1`, id).Scan(
			&currentStatus,
			&currentReason,
		)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.ErrPaymentAttemptNotFound
			}
			return fmt.Errorf("get payment attempt after status update: %w", err)
		}

		if currentStatus == targetStatus {
			switch targetStatus {
			case models.AttemptStatusProcessing:
				// do nothing on double processing
				return nil

			case models.AttemptStatusFailed, models.AttemptStatusCanceled:
				if *currentReason == reason {
					return nil
				}
			}
		}
		return models.ErrPaymentAttemptValidation
	}

	return nil
}

package paymentrepository

import (
	"context"
	"errors"
	"fmt"
	"time"

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
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`

	var id uuid.UUID
	var createdAt, updatedAt time.Time
	err := r.pool.QueryRow(ctx, query, attempt.PaymentOrderID, attempt.Provider, attempt.Status).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		err = postgres.ParseError(err)
		switch {
		case errors.Is(err, postgres.ErrFKViolation):
			return nil, models.ErrPaymentAttemptValidation
		case errors.Is(err, postgres.ErrCheckViolation):
			return nil, models.ErrPaymentAttemptValidation
		}
		return nil, fmt.Errorf("create payment attempt error: %w", err)
	}

	attempt.ID = id
	attempt.CreatedAt = createdAt
	attempt.UpdatedAt = updatedAt
	return attempt, nil
}

func (r *PaymentAttemptRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PaymentAttempt, error) {
	const query = `SELECT ` + paymentAttemptColumns + ` FROM payment_attempt WHERE id = $1`

	attempt := &models.PaymentAttempt{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&attempt.ID, &attempt.PaymentOrderID, &attempt.Provider,
		&attempt.ExternalTransactionID, &attempt.Status,
		&attempt.ChargedAmountMinor, &attempt.ChargedCurrency,
		&attempt.FailureReason, &attempt.CompletedAt,
		&attempt.CreatedAt, &attempt.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrPaymentAttemptNotFound
		}
		return nil, fmt.Errorf("get payment attempt error: %w", err)
	}
	return attempt, nil
}

func (r *PaymentAttemptRepository) Update(ctx context.Context, attempt *models.PaymentAttempt) (*models.PaymentAttempt, error) {
	const query = `
		UPDATE payment_attempt 
		SET external_transaction_id = $2, status = $3, charged_amount_minor = $4, 
		    charged_currency = $5, failure_reason = $6, completed_at = $7
		WHERE id = $1
		RETURNING updated_at
	`

	var updatedAt time.Time
	err := r.pool.QueryRow(ctx, query,
		attempt.ID, attempt.ExternalTransactionID, attempt.Status,
		attempt.ChargedAmountMinor, attempt.ChargedCurrency,
		attempt.FailureReason, attempt.CompletedAt,
	).Scan(&updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrPaymentAttemptNotFound
		}
		err = postgres.ParseError(err)
		if errors.Is(err, postgres.ErrCheckViolation) || errors.Is(err, postgres.ErrUniqViolation) {
			return nil, models.ErrPaymentAttemptValidation
		}
		return nil, fmt.Errorf("update payment attempt error: %w", err)
	}

	attempt.UpdatedAt = updatedAt
	return attempt, nil
}

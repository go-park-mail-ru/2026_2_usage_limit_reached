package paymentrepository

import (
	"context"
	"errors"
	"fmt"

	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PaymentAttemptRepository struct {
	pool postgres.DB
}

func NewPaymentAttemptRepository(db postgres.DBClient) *PaymentAttemptRepository {
	return &PaymentAttemptRepository{pool: db.Pool}
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

func (r *PaymentAttemptRepository) GetAttemptByID(ctx context.Context, id uuid.UUID) (*models.PaymentAttempt, error) {
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

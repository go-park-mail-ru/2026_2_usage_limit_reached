package paymentrepository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PaymentRepository) GetAttemptByID(ctx context.Context, id uuid.UUID) (*models.PaymentAttempt, error) {
	const query = `
		SELECT 
			id, 
			payment_order_id, 
			provider, 
			external_transaction_id, 
			status, 
			charged_amount_minor, 
			charged_currency, 
			failure_reason, 
			completed_at, 
			created_at, 
			updated_at 
		FROM payment_attempt WHERE id = $1
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get attempt by ID"),
		slog.String("id", id.String()),
	)

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

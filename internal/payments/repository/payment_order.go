package paymentrepository

import (
	"context"
	"errors"
	"fmt"

	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type PgxCliPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
}

type PaymentOrderRepository struct {
	pool PgxCliPool
}

func NewPaymentOrderRepository(pool PgxCliPool) *PaymentOrderRepository {
	return &PaymentOrderRepository{pool: pool}
}

func (r *PaymentOrderRepository) MarkPaid(ctx context.Context, orderID, attemptID uuid.UUID) error {
	const query = `
		UPDATE payment_order 
		SET status = 'paid', paid_at = now(), credited_attempt_id = $2
		WHERE id = $1 AND status IN ('pending', 'processing')
	`

	result, err := r.pool.Exec(ctx, query, orderID, attemptID)
	if err != nil {
		err = postgres.ParseError(err)
		if errors.Is(err, postgres.ErrFKViolation) {
			return models.ErrPaymentOrderValidation
		}
		return fmt.Errorf("mark order paid: %w", err)
	}
	if result.RowsAffected() == 0 {
		var currentStatus models.PaymentOrderStatus
		err = r.pool.QueryRow(ctx, `SELECT status FROM payment_order WHERE id = $1`, orderID).Scan(&currentStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrPaymentOrderNotFound
		}
		if currentStatus == models.StatusPaid {
			return nil // already paid, do nothing
		}
		return models.ErrPaymentOrderValidation
	}
	return nil
}

func (r *PaymentOrderRepository) Cancel(ctx context.Context, orderID uuid.UUID) error {
	const query = `
		UPDATE payment_order 
		SET status = 'canceled', canceled_at = now()
		WHERE id = $1 AND status IN ('pending', 'processing')
	`
	result, err := r.pool.Exec(ctx, query, orderID)
	if err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}
	if result.RowsAffected() == 0 {
		var currentStatus models.PaymentOrderStatus
		err = r.pool.QueryRow(ctx, `SELECT status FROM payment_order WHERE id = $1`, orderID).Scan(&currentStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrPaymentOrderNotFound
		}
		if currentStatus == models.StatusPaid || currentStatus == models.StatusCanceled {
			return models.ErrForbiddenUpdate
		}
		return models.ErrPaymentOrderValidation
	}
	return nil
}

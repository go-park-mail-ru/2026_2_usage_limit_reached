package paymentrepository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/dto"
	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	reasonDuplicatePayment = "duplicate_payment"
	reasonOrderCanceled    = "order_canceled"
	reasonAmountMismatch   = "amount_mismatch"
)

type PaymentRepository struct {
	pool   postgres.DB
	logger *slog.Logger
}

func NewPaymentRepository(db *postgres.DBClient, logger *slog.Logger) *PaymentRepository {
	return &PaymentRepository{pool: db.Pool, logger: logger}
}

func (r *PaymentRepository) GetOrderByID(ctx context.Context, id uuid.UUID) (*models.PaymentOrder, error) {
	const query = `
		SELECT
			id, payer_id, request_key, purpose, origin, expected_amount_minor, currency, status, 
			credited_attempt_id, expires_at, paid_at, canceled_at, created_at, updated_at
		FROM payment_order WHERE id = $1
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get order by ID"),
		slog.String("id", id.String()),
	)

	order := &models.PaymentOrder{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
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

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrPaymentOrderNotFound
		}
		return nil, fmt.Errorf("get payment order error: %w", err)
	}

	return order, nil
}

func (r *PaymentRepository) CompleteSuccessfulAttempt(ctx context.Context, attempt dto.AttemptToComplete) (bool, error) {
	payment := models.SuccessfulPayment{
		AttemptID:             attempt.ID,
		OrderID:               attempt.OrderID,
		ExternalTransactionID: attempt.ExternalTransactionID,
		ChargedAmount:         attempt.ChargedAmountMinor,
		ChargedCurrency:       attempt.ChargedCurrency,
	}

	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "complete successful attempt"),
		slog.String("order_id", payment.OrderID.String()),
		slog.String("attempt_id", payment.AttemptID.String()),
		slog.String("external_transaction_id", payment.ExternalTransactionID),
	)

	requireRefund := false
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		order, err := lockOrder(ctx, tx, payment.OrderID)
		if err != nil {
			return err
		}

		attemptState, err := lockAttempt(ctx, tx, payment.AttemptID)
		if err != nil {
			return err
		}

		d, err := models.MakeOrderDecision(order, attemptState, payment)
		if err != nil {
			return err
		}

		if err := r.applyDecision(ctx, tx, d, attempt, payment); err != nil {
			return err
		}

		requireRefund = d.RequiresRefund()
		return nil
	})
	if err != nil {
		return false, mapCompleteError(err)
	}

	return requireRefund, nil
}

func lockOrder(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (models.OrderState, error) {
	const query = `
		SELECT status, expected_amount_minor, currency, credited_attempt_id
		FROM payment_order
		WHERE id = $1
		FOR UPDATE
	`

	var orderState models.OrderState
	err := tx.QueryRow(ctx, query, orderID).Scan(
		&orderState.Status,
		&orderState.ExpectedAmount,
		&orderState.ExpectedCurrency,
		&orderState.CreditedAttemptID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.OrderState{}, models.ErrPaymentOrderNotFound
	}
	if err != nil {
		return models.OrderState{}, fmt.Errorf("lock payment order: %w", err)
	}
	return orderState, nil
}

func lockAttempt(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID) (models.AttemptState, error) {
	const query = `
		SELECT payment_order_id, status, external_transaction_id, charged_amount_minor, charged_currency
		FROM payment_attempt
		WHERE id = $1
		FOR UPDATE
	`

	var attemptState models.AttemptState
	err := tx.QueryRow(ctx, query, attemptID).Scan(
		&attemptState.OrderID,
		&attemptState.Status,
		&attemptState.ExternalTransactionID,
		&attemptState.ChargedAmount,
		&attemptState.ChargedCurrency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.AttemptState{}, models.ErrPaymentAttemptNotFound
	}
	if err != nil {
		return models.AttemptState{}, fmt.Errorf("lock payment attempt: %w", err)
	}
	return attemptState, nil
}

func (r *PaymentRepository) applyDecision(ctx context.Context, tx pgx.Tx, d models.OrderDecision, attempt dto.AttemptToComplete, p models.SuccessfulPayment) error {
	if d.SaveAttempt {
		if err := saveSuccessfulAttempt(ctx, tx, attempt); err != nil {
			return err
		}
	}
	if d.FailOrder {
		if err := failOrder(ctx, tx, p.OrderID); err != nil {
			return err
		}
	}
	if d.Refund != "" {
		if err := r.makeRefund(ctx, tx, p.AttemptID, p.ChargedAmount, d.Refund); err != nil {
			return err
		}
	}
	if d.MarkPaid {
		return markOrderPaid(ctx, tx, attempt)
	}

	return nil
}

func mapCompleteError(err error) error {
	if errors.Is(err, models.ErrPaymentOrderNotFound) ||
		errors.Is(err, models.ErrPaymentAttemptNotFound) ||
		errors.Is(err, models.ErrPaymentOrderValidation) ||
		errors.Is(err, models.ErrPaymentAttemptValidation) {
		return err
	}

	parsed := postgres.ParseError(err)
	if errors.Is(parsed, postgres.ErrFKViolation) ||
		errors.Is(parsed, postgres.ErrCheckViolation) ||
		errors.Is(parsed, postgres.ErrUniqViolation) {
		return models.ErrPaymentOrderValidation
	}

	return fmt.Errorf("complete successful payment attempt: %w", err)
}

func saveSuccessfulAttempt(ctx context.Context, tx pgx.Tx, attempt dto.AttemptToComplete) error {
	const updateAttempt = `
		UPDATE payment_attempt
		SET external_transaction_id = $3,
			status = 'succeeded',
			charged_amount_minor = $4,
			charged_currency = $5,
			failure_reason = NULL,
			completed_at = $6
		WHERE id = $1 AND payment_order_id = $2 AND status IN ('created', 'processing', 'failed', 'canceled')
	`

	result, err := tx.Exec(ctx, updateAttempt,
		attempt.ID,
		attempt.OrderID,
		attempt.ExternalTransactionID,
		attempt.ChargedAmountMinor,
		attempt.ChargedCurrency,
		attempt.CompletedAt,
	)
	if err != nil {
		parsed := postgres.ParseError(err)
		if errors.Is(parsed, postgres.ErrFKViolation) ||
			errors.Is(parsed, postgres.ErrCheckViolation) ||
			errors.Is(parsed, postgres.ErrUniqViolation) {
			return models.ErrPaymentAttemptValidation
		}
		return fmt.Errorf("mark payment attempt succeeded: %w", err)
	}
	if result.RowsAffected() != 1 {
		return models.ErrPaymentAttemptValidation
	}
	return nil
}

func markOrderPaid(ctx context.Context, tx pgx.Tx, attempt dto.AttemptToComplete) error {
	const markAsPaid = `
		UPDATE payment_order
		SET status = 'paid', paid_at = now(), credited_attempt_id = $2
		WHERE id = $1
			AND status IN ('pending', 'processing', 'failed')
			AND expected_amount_minor = $3
			AND currency = $4
	`

	result, err := tx.Exec(ctx, markAsPaid,
		attempt.OrderID,
		attempt.ID,
		attempt.ChargedAmountMinor,
		attempt.ChargedCurrency,
	)
	if err != nil {
		parsed := postgres.ParseError(err)
		if errors.Is(parsed, postgres.ErrFKViolation) ||
			errors.Is(parsed, postgres.ErrCheckViolation) ||
			errors.Is(parsed, postgres.ErrUniqViolation) {
			return models.ErrPaymentOrderValidation
		}
		return fmt.Errorf("mark payment order paid: %w", err)
	}
	if result.RowsAffected() != 1 {
		return models.ErrPaymentOrderValidation
	}
	return nil
}

func (r *PaymentRepository) makeRefund(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID, amount int64, reason models.RefundReason) error {
	const query = `
		INSERT INTO refund (payment_attempt_id, amount_minor, kind, reason)
		VALUES ($1, $2, 'excess_charge', $3)
		ON CONFLICT (payment_attempt_id) DO NOTHING
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "make refund"),
		slog.String("attemptID", attemptID.String()),
		slog.Int64("amount", amount),
		slog.String("reason", string(reason)),
	)

	if _, err := tx.Exec(ctx, query, attemptID, amount, reason); err != nil {
		return fmt.Errorf("insert refund: %w", err)
	}
	return nil
}

func (r *PaymentRepository) FailAttempt(ctx context.Context, orderID, attemptID uuid.UUID, reason string) error {
	const lockOrder = `SELECT status FROM payment_order WHERE id = $1 FOR UPDATE`
	const hasActiveQuery = `
		SELECT EXISTS (
			SELECT 1 FROM payment_attempt
			WHERE payment_order_id = $1 AND status IN ('created', 'processing')
		)
	`

	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "fail attempt"),
		slog.String("order_id", orderID.String()),
		slog.String("attempt_id", attemptID.String()),
		slog.String("reason", reason),
	)

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		var status models.PaymentOrderStatus
		err := tx.QueryRow(ctx, lockOrder, orderID).Scan(&status)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.ErrPaymentOrderNotFound
			}
			return fmt.Errorf("lock payment order: %w", err)
		}

		if err := markAttemptFailed(ctx, tx, orderID, attemptID, reason); err != nil {
			return err
		}

		var hasActive bool
		if err := tx.QueryRow(ctx, hasActiveQuery, orderID).Scan(&hasActive); err != nil {
			return fmt.Errorf("check active attempts: %w", err)
		}
		if hasActive {
			return nil
		}
		return failOrder(ctx, tx, orderID)
	})
	return err
}

func failOrder(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	const query = `UPDATE payment_order SET status = 'failed' WHERE id = $1 AND status IN ('pending', 'processing')`
	if _, err := tx.Exec(ctx, query, orderID); err != nil {
		parsed := postgres.ParseError(err)
		if errors.Is(parsed, postgres.ErrCheckViolation) || errors.Is(parsed, postgres.ErrFKViolation) {
			return models.ErrPaymentOrderValidation
		}
		return fmt.Errorf("mark payment order failed: %w", err)
	}
	return nil
}

func markAttemptFailed(ctx context.Context, tx pgx.Tx, orderID, attemptID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return models.ErrPaymentAttemptValidation
	}

	return changeAttemptStatus(ctx, tx, orderID, attemptID, models.AttemptStatusFailed, reason)
}

func changeAttemptStatus(ctx context.Context, tx pgx.Tx, orderID, attemptID uuid.UUID, targetStatus models.PaymentAttemptStatus, reason string) error {
	var (
		query string
		args  []any
	)

	switch targetStatus { // на будущее, пока только failed
	case models.AttemptStatusFailed:
		query = `
			UPDATE payment_attempt
			SET status = 'failed', failure_reason = $3
			WHERE id = $1 AND payment_order_id = $2 AND status IN ('created', 'processing')
		`
		args = []any{attemptID, orderID, reason}
	default:
		return models.ErrPaymentAttemptValidation
	}

	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("change payment attempt status: %w", err)
	}

	if result.RowsAffected() != 1 {
		var (
			currentStatus models.PaymentAttemptStatus
			currentReason *string
		)

		err = tx.QueryRow(ctx, `SELECT status, failure_reason FROM payment_attempt WHERE id = $1 AND payment_order_id = $2`, attemptID, orderID).Scan(
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
				// ничего не делаем на повторный processing
				return nil

			case models.AttemptStatusFailed, models.AttemptStatusCanceled:
				if currentReason != nil && *currentReason == reason {
					return nil
				}
			}
		}
		return models.ErrPaymentAttemptValidation
	}

	return nil
}

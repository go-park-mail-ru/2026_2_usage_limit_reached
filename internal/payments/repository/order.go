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
	const lockOrder = `
		SELECT status, expected_amount_minor, currency, credited_attempt_id
		FROM payment_order
		WHERE id = $1
		FOR UPDATE
	`
	const lockAttempt = `
		SELECT payment_order_id, status, external_transaction_id, charged_amount_minor, charged_currency
		FROM payment_attempt
		WHERE id = $1
		FOR UPDATE
	`

	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "get order by ID"),
		slog.String("order_id", attempt.OrderID.String()),
		slog.String("attempt_id", attempt.ID.String()),
		slog.String("external_transaction_id", attempt.ExternalTransactionID),
	)

	requireRefund := false
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		var (
			orderStatus       models.PaymentOrderStatus
			expectedAmount    int64
			expectedCurrency  string
			creditedAttemptID *uuid.UUID
		)
		err := tx.QueryRow(ctx, lockOrder, attempt.OrderID).Scan(
			&orderStatus,
			&expectedAmount,
			&expectedCurrency,
			&creditedAttemptID,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.ErrPaymentOrderNotFound
			}
			return fmt.Errorf("lock payment order: %w", err)
		}

		var (
			attemptOrderID  uuid.UUID
			attemptStatus   models.PaymentAttemptStatus
			externalID      *string
			chargedAmount   *int64
			chargedCurrency *string
		)
		err = tx.QueryRow(ctx, lockAttempt, attempt.ID).Scan(
			&attemptOrderID,
			&attemptStatus,
			&externalID,
			&chargedAmount,
			&chargedCurrency,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.ErrPaymentAttemptNotFound
			}
			return fmt.Errorf("lock payment attempt: %w", err)
		}

		alreadySucceeded := false
		if attemptStatus == models.AttemptStatusSucceeded {
			same := externalID != nil && *externalID == attempt.ExternalTransactionID &&
				chargedAmount != nil && *chargedAmount == attempt.ChargedAmountMinor &&
				chargedCurrency != nil && *chargedCurrency == attempt.ChargedCurrency
			if attemptOrderID != attempt.OrderID || !same {
				return models.ErrPaymentAttemptValidation
			}
			alreadySucceeded = true
		}

		if !alreadySucceeded {
			if err := saveSuccessfulAttempt(ctx, tx, attempt); err != nil {
				return err
			}
		}

		switch orderStatus {
		case models.StatusPaid: // возврат на уже оплаченный заказ
			requireRefund = *creditedAttemptID != attempt.ID
			if requireRefund {
				return r.makeRefund(ctx, tx, attempt.ID, attempt.ChargedAmountMinor, reasonDuplicatePayment)
			}
			return nil
		case models.StatusCanceled: // возврат на отмененный заказ
			requireRefund = true
			return r.makeRefund(ctx, tx, attempt.ID, attempt.ChargedAmountMinor, reasonOrderCanceled)
		}

		if expectedAmount != attempt.ChargedAmountMinor || expectedCurrency != attempt.ChargedCurrency { // fail invalid order
			if orderStatus == models.StatusPending || orderStatus == models.StatusProcessing {
				if err := failOrder(ctx, tx, attempt.OrderID); err != nil {
					return err
				}
			}
			requireRefund = true
			return r.makeRefund(ctx, tx, attempt.ID, attempt.ChargedAmountMinor, reasonAmountMismatch)
		}

		if orderStatus != models.StatusPending &&
			orderStatus != models.StatusProcessing &&
			orderStatus != models.StatusFailed {
			return models.ErrPaymentOrderValidation
		}

		return markOrderPaid(ctx, tx, attempt)
	})
	if err != nil {
		if errors.Is(err, models.ErrPaymentOrderNotFound) ||
			errors.Is(err, models.ErrPaymentAttemptNotFound) ||
			errors.Is(err, models.ErrPaymentOrderValidation) ||
			errors.Is(err, models.ErrPaymentAttemptValidation) {
			return false, err
		}

		parsed := postgres.ParseError(err)
		if errors.Is(parsed, postgres.ErrFKViolation) ||
			errors.Is(parsed, postgres.ErrCheckViolation) ||
			errors.Is(parsed, postgres.ErrUniqViolation) {
			return false, models.ErrPaymentOrderValidation
		}

		return false, fmt.Errorf("complete successful payment attempt: %w", err)
	}
	return requireRefund, nil
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

func (r *PaymentRepository) makeRefund(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID, amount int64, reason string) error {
	const query = `
		INSERT INTO refund (payment_attempt_id, amount_minor, kind, reason)
		VALUES ($1, $2, 'excess_charge', $3)
		ON CONFLICT (payment_attempt_id) DO NOTHING
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "make refund"),
		slog.String("attemptID", attemptID.String()),
		slog.Int64("amount", amount),
		slog.String("reason", reason),
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
				// do nothing on double processing
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

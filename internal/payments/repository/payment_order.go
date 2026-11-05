package paymentrepository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/dto"
	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	reasonDuplicatePayment = "duplicate_payment"
	reasonOrderCanceled = "order_canceled"
	reasonAmountMismatch = "amount_mismatch"
)

type PaymentOrderRepository struct {
	pool postgres.DB
}

func NewPaymentOrderRepository(db postgres.DBClient) *PaymentOrderRepository {
	return &PaymentOrderRepository{pool: db.Pool}
}

func (r *PaymentOrderRepository) GetOrderByID(ctx context.Context, id uuid.UUID) (*models.PaymentOrder, error) {
	const query = `
		SELECT
			id, payer_id, request_key, purpose, origin, expected_amount_minor, currency, status, 
			credited_attempt_id, expires_at, paid_at, canceled_at, created_at, updated_at
		FROM payment_order WHERE id = $1
	`

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

func (r *PaymentOrderRepository) CompleteSuccessfulAttempt(ctx context.Context, attempt dto.AttemptToComplete) (bool, error) {
	requireRefund := false
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const lockOrder = `
			SELECT status, expected_amount_minor, currency, credited_attempt_id
			FROM payment_order
			WHERE id = $1
			FOR UPDATE
		`
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

		const lockAttempt = `
			SELECT payment_order_id, status, external_transaction_id, charged_amount_minor, charged_currency
			FROM payment_attempt
			WHERE id = $1
			FOR UPDATE
		`
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
		case models.StatusPaid: // refund on already paid order
			requireRefund = *creditedAttemptID != attempt.ID
			if requireRefund {
				return makeRefund(ctx, tx, attempt.ID, attempt.ChargedAmountMinor, reasonDuplicatePayment)
			}
			return nil
		case models.StatusCanceled: // refund on calceled order
			requireRefund = true
			return makeRefund(ctx, tx, attempt.ID, attempt.ChargedAmountMinor, reasonOrderCanceled)
		}

		if expectedAmount != attempt.ChargedAmountMinor || expectedCurrency != attempt.ChargedCurrency { // fail invalid order
			if orderStatus == models.StatusPending || orderStatus == models.StatusProcessing {
				if err := failOrder(ctx, tx, attempt.OrderID); err != nil {
					return err
				}
			}
			requireRefund = true
			return makeRefund(ctx, tx, attempt.ID, attempt.ChargedAmountMinor, reasonAmountMismatch)
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

func makeRefund(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID, amount int64, reason string) error {
	const query = `
		INSERT INTO refund (payment_attempt_id, amount_minor, kind, reason)
		VALUES ($1, $2, 'excess_charge', $3)
		ON CONFLICT (payment_attempt_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, query, attemptID, amount, reason); err != nil {
		return fmt.Errorf("insert refund: %w", err)
	}
	return nil
}

func (r *PaymentOrderRepository) FailAttempt(ctx context.Context, orderID, attemptID uuid.UUID, reason string) error {
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		var status models.PaymentOrderStatus
		err := tx.QueryRow(ctx, `SELECT status FROM payment_order WHERE id = $1 FOR UPDATE`, orderID).Scan(&status)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.ErrPaymentOrderNotFound
			}
			return fmt.Errorf("lock payment order: %w", err)
		}

		if err := markAttemptFailed(ctx, tx, orderID, attemptID, reason); err != nil {
			return err
		}

		const hasActiveQuery = `
			SELECT EXISTS (
				SELECT 1 FROM payment_attempt
				WHERE payment_order_id = $1 AND status IN ('created', 'processing')
			)
		`
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

func (r *PaymentOrderRepository) Cancel(ctx context.Context, userID, orderID uuid.UUID, failureReason string) error {
	failureReason = strings.TrimSpace(failureReason)
	if failureReason == "" {
		failureReason = reasonOrderCanceled
	}
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		var canceledID uuid.UUID
		const cancelOrder = `
			UPDATE payment_order
			SET status = 'canceled', canceled_at = now()
			WHERE id = $1 AND payer_id = $2 AND status IN ('pending', 'processing', 'failed')
			RETURNING id
		`
		err := tx.QueryRow(ctx, cancelOrder, orderID, userID).Scan(&canceledID)
		if errors.Is(err, pgx.ErrNoRows) {
			var (
				currentStatus models.PaymentOrderStatus
				payerID       uuid.UUID
			)
			err = tx.QueryRow(ctx, `SELECT status, payer_id FROM payment_order WHERE id = $1`, orderID).Scan(&currentStatus, &payerID)
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				return models.ErrPaymentOrderNotFound
			case err != nil:
				return fmt.Errorf("get payment order status: %w", err)
			case payerID != userID:
				return models.ErrPaymentOrderNotFound
			case currentStatus == models.StatusPaid || currentStatus == models.StatusCanceled:
				return models.ErrForbiddenUpdate
			}
			return models.ErrPaymentOrderValidation
		}
		if err != nil {
			parsed := postgres.ParseError(err)
			if errors.Is(parsed, postgres.ErrCheckViolation) {
				return models.ErrPaymentOrderValidation
			}
			return fmt.Errorf("cancel payment order: %w", err)
		}

		return cancelActiveAttempts(ctx, tx, orderID, failureReason)
	})

	if err != nil {
		if errors.Is(err, models.ErrPaymentOrderNotFound) ||
			errors.Is(err, models.ErrForbiddenUpdate) ||
			errors.Is(err, models.ErrPaymentOrderValidation) {
			return err
		}

		parsed := postgres.ParseError(err)
		if errors.Is(parsed, postgres.ErrCheckViolation) ||
			errors.Is(parsed, postgres.ErrFKViolation) {
			return models.ErrPaymentOrderValidation
		}

		return fmt.Errorf("cancel payment order transaction: %w", err)
	}

	return nil
}

func cancelActiveAttempts(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return models.ErrPaymentAttemptValidation
	}

	const query = `
		UPDATE payment_attempt
		SET status = 'canceled', failure_reason = $2
		WHERE payment_order_id = $1 AND status IN ('created', 'processing')
	`
	if _, err := tx.Exec(ctx, query, orderID, reason); err != nil {
		parsed := postgres.ParseError(err)
		if errors.Is(parsed, postgres.ErrCheckViolation) {
			return models.ErrPaymentAttemptValidation
		}
		return fmt.Errorf("cancel payment attempts: %w", err)
	}
	return nil
}

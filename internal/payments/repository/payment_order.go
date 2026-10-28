package paymentrepository

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/dto"
	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type PgxCliPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type PaymentOrderRepository struct {
	pool PgxCliPool
}

func NewPaymentOrderRepository(pool PgxCliPool) *PaymentOrderRepository {
	return &PaymentOrderRepository{pool: pool}
}

func (r *PaymentOrderRepository) CompleteAttempt(ctx context.Context, attempt dto.AttemptToComplete) (bool, error) {
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
		}

		switch orderStatus {
		case models.StatusPaid: // refund on already paid order
			requireRefund = *creditedAttemptID != attempt.ID
			if requireRefund {
				return makeRefund(ctx, tx, attempt.ID, attempt.ChargedAmountMinor, "duplicate_payment")
			}
			return nil
		case models.StatusCanceled: // refund on calceled order
			requireRefund = true
			return makeRefund(ctx, tx, attempt.ID, attempt.ChargedAmountMinor, "order_canceled")
		}

		if expectedAmount != attempt.ChargedAmountMinor || expectedCurrency != attempt.ChargedCurrency { // fail invalid order
			if orderStatus == models.StatusPending || orderStatus == models.StatusProcessing {
				const failOrder = `
				UPDATE payment_order
				SET status = 'failed'
				WHERE id = $1 AND status IN ('pending', 'processing')
			`
				result, err := tx.Exec(ctx, failOrder, attempt.OrderID)
				if err != nil {
					parsed := postgres.ParseError(err)
					if errors.Is(parsed, postgres.ErrCheckViolation) || errors.Is(parsed, postgres.ErrFKViolation) {
						return models.ErrPaymentOrderValidation
					}
					return fmt.Errorf("mark invalid payment order failed: %w", err)
				}
				if result.RowsAffected() != 1 {
					return models.ErrPaymentOrderValidation
				}
			}
			requireRefund = true
			return makeRefund(ctx, tx, attempt.ID, attempt.ChargedAmountMinor, "amount_mismatch")
		}

		switch orderStatus {
		case models.StatusPending, models.StatusProcessing, models.StatusFailed:
			// can pay from these statuses
		default:
			return models.ErrPaymentOrderValidation
		}

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

func (r *PaymentOrderRepository) Cancel(ctx context.Context, userID, orderID uuid.UUID, failureReason string) error {
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
			var currentStatus models.PaymentOrderStatus
			err = tx.QueryRow(ctx, `SELECT status FROM payment_order WHERE id = $1`, orderID).Scan(&currentStatus)
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				return models.ErrPaymentOrderNotFound
			case err != nil:
				return fmt.Errorf("get payment order status: %w", err)
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

		const cancelAttempt = `
			UPDATE payment_attempt
			SET status = 'canceled', failure_reason = $2
			WHERE payment_order_id = $1 AND status IN ('created', 'processing')
		`
		_, err = tx.Exec(ctx, cancelAttempt, orderID, failureReason)
		if err != nil {
			parsed := postgres.ParseError(err)
			if errors.Is(parsed, postgres.ErrCheckViolation) ||
				errors.Is(parsed, postgres.ErrFKViolation) {
				return models.ErrPaymentOrderValidation
			}
			return fmt.Errorf("cancel payment attempts: %w", err)
		}

		return nil
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

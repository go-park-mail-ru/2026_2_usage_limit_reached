package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/dto"
	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	reasonDuplicatePayment = "duplicate_payment"
	reasonOrderCanceled    = "order_canceled"
	reasonOrderExpired     = "order_expired"
	reasonAmountMismatch   = "amount_mismatch"

	defaultPaymentProvider    = "mock"
	defaultExpiredOrdersLimit = 100
)

type PaymentRepository struct {
	pool   postgres.DB
	logger *slog.Logger
}

func NewPaymentRepository(db *postgres.DBClient, logger *slog.Logger) *PaymentRepository {
	return &PaymentRepository{pool: db.Pool, logger: logger}
}

func (r *PaymentRepository) CreateOrGetOrder(ctx context.Context, req dto.CreatePayment) (*models.PaymentOrder, error) {
	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "create or get payment order"),
		slog.String("payerID", req.PayerID.String()),
		slog.String("requestKey", req.RequestKey),
		slog.String("purpose", req.Purpose),
	)

	order := &models.PaymentOrder{}

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const insertOrder = `
			INSERT INTO payment_order (
				payer_id,
				request_key,
				purpose,
				origin,
				expected_amount_minor,
				currency,
				status,
				expires_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
			ON CONFLICT (request_key) DO NOTHING
			RETURNING 
				id,
				payer_id,
				request_key,
				purpose,
				origin,
				expected_amount_minor,
				currency,
				status,
				credited_attempt_id,
				expires_at,
				paid_at,
				canceled_at,
				created_at,
				updated_at
		`

		err := tx.QueryRow(
			ctx,
			insertOrder,
			req.PayerID,
			req.RequestKey,
			req.Purpose,
			req.Origin,
			req.AmountMinor,
			req.Currency,
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

		switch {
		case err == nil:
			err = createInitialAttempt(ctx, tx, order.ID)
			return err
		case errors.Is(err, pgx.ErrNoRows):
			order, err = getExistingOrder(ctx, tx, req)
			return err
		default:
			parsedErr := postgres.ParseError(err)
			if errors.Is(parsedErr, postgres.ErrFKViolation) ||
				errors.Is(parsedErr, postgres.ErrCheckViolation) ||
				errors.Is(parsedErr, postgres.ErrUniqViolation) {
				return models.ErrPaymentOrderValidation
			}

			return fmt.Errorf("insert payment order: %w", err)
		}
	})

	if err != nil {
		parsedErr := postgres.ParseError(err)
		if errors.Is(parsedErr, postgres.ErrFKViolation) ||
			errors.Is(parsedErr, postgres.ErrCheckViolation) ||
			errors.Is(parsedErr, postgres.ErrUniqViolation) {
			return nil, models.ErrPaymentOrderValidation
		}

		return nil, fmt.Errorf("create payment transaction: %w", err)
	}

	return order, nil
}

func createInitialAttempt(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	const query = `
		INSERT INTO payment_attempt (payment_order_id, provider, status)
		VALUES ($1, $2, 'created')
	`

	_, err := tx.Exec(ctx, query, orderID, defaultPaymentProvider)
	if err != nil {
		parsedErr := postgres.ParseError(err)
		if errors.Is(parsedErr, postgres.ErrFKViolation) ||
			errors.Is(parsedErr, postgres.ErrCheckViolation) ||
			errors.Is(parsedErr, postgres.ErrUniqViolation) {
			return models.ErrPaymentAttemptValidation
		}

		return fmt.Errorf("create initial payment attempt: %w", err)
	}

	return nil
}

func getExistingOrder(ctx context.Context, tx pgx.Tx, req dto.CreatePayment) (*models.PaymentOrder, error) {
	const getOrder = `
		SELECT
			id,
			payer_id,
			request_key,
			purpose,
			origin,
			expected_amount_minor,
			currency,
			status,
			credited_attempt_id,
			expires_at,
			paid_at,
			canceled_at,
			created_at,
			updated_at
		FROM payment_order
		WHERE request_key = $1
		FOR UPDATE
	`

	order := &models.PaymentOrder{}
	err := tx.QueryRow(ctx, getOrder, req.RequestKey).Scan(
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
		return nil, fmt.Errorf("get existing payment order: %w", err)
	}

	if order.PayerID != req.PayerID ||
		string(order.Purpose) != req.Purpose ||
		string(order.Origin) != req.Origin ||
		order.ExpectedAmountMinor != req.AmountMinor ||
		order.Currency != req.Currency ||
		!equalOptionalTime(order.ExpiresAt, req.ExpiresAt) {
		return nil, models.ErrPaymentOrderValidation
	}

	const getLatestAttempt = `
		SELECT id
		FROM payment_attempt
		WHERE payment_order_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`

	var attemptID uuid.UUID
	err = tx.QueryRow(ctx, getLatestAttempt, order.ID).Scan(&attemptID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrPaymentAttemptNotFound
		}
		return nil, fmt.Errorf("get latest payment attempt: %w", err)
	}

	return order, nil
}

func (r *PaymentRepository) StartAttempt(ctx context.Context, orderID uuid.UUID) (*models.PaymentOrder, *models.PaymentAttempt, bool, error) {
	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "start payment attempt"),
		slog.String("order_id", orderID.String()),
	)

	order := &models.PaymentOrder{}
	attempt := &models.PaymentAttempt{}
	started := false

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const lockOrder = `
            SELECT
                id,
                payer_id,
                request_key,
                purpose,
                origin,
                expected_amount_minor,
                currency,
                status,
                credited_attempt_id,
                expires_at,
                paid_at,
                canceled_at,
                created_at,
                updated_at
            FROM payment_order
            WHERE id = $1
            FOR UPDATE
        `

		err := tx.QueryRow(ctx, lockOrder, orderID).Scan(
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
				return models.ErrPaymentOrderNotFound
			}
			return fmt.Errorf("lock payment order: %w", err)
		}

		const lockAttempt = `
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
            FROM payment_attempt
            WHERE payment_order_id = $1
            ORDER BY created_at DESC, id DESC
            LIMIT 1
            FOR UPDATE
        `

		err = tx.QueryRow(ctx, lockAttempt, orderID).Scan(
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
				return models.ErrPaymentAttemptNotFound
			}
			return fmt.Errorf("lock payment attempt: %w", err)
		}

		if order.Status != models.StatusPending ||
			attempt.Status != models.AttemptStatusCreated {
			return nil
		}

		const startOrder = `
            UPDATE payment_order
            SET status = 'processing'
            WHERE id = $1
              AND status = 'pending'
              AND (expires_at IS NULL OR expires_at > clock_timestamp())
        `

		result, err := tx.Exec(ctx, startOrder, orderID)
		if err != nil {
			return fmt.Errorf("mark payment order processing: %w", err)
		}

		// Заказ уже истек. Впоследствии его отменит expiration worker.
		if result.RowsAffected() != 1 {
			return nil
		}

		const startAttempt = `
            UPDATE payment_attempt
            SET status = 'processing'
            WHERE id = $1
              AND payment_order_id = $2
              AND status = 'created'
        `

		result, err = tx.Exec(ctx, startAttempt, attempt.ID, orderID)
		if err != nil {
			return fmt.Errorf("mark payment attempt processing: %w", err)
		}

		if result.RowsAffected() != 1 {
			return models.ErrPaymentAttemptValidation
		}

		order.Status = models.StatusProcessing
		attempt.Status = models.AttemptStatusProcessing
		started = true

		return nil
	})

	if err != nil {
		return nil, nil, false, fmt.Errorf("start payment attempt: %w", err)
	}

	return order, attempt, started, nil
}

func (r *PaymentRepository) GetOrdersByIDs(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]*models.PaymentOrder, error) {
	orderIDs = uniqueUUIDs(orderIDs)

	if len(orderIDs) == 0 {
		return map[uuid.UUID]*models.PaymentOrder{}, nil
	}

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get orders"),
		slog.Int("count", len(orderIDs)),
	)

	const query = `
		SELECT
			id,
			payer_id,
			request_key,
			purpose,
			origin,
			expected_amount_minor,
			currency,
			status,
			credited_attempt_id,
			expires_at,
			paid_at,
			canceled_at,
			created_at,
			updated_at
		FROM payment_order
		WHERE id = ANY($1::uuid[])
	`

	orders := make(map[uuid.UUID]*models.PaymentOrder, len(orderIDs))

	rows, err := r.pool.Query(ctx, query, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("get payment orders: %w", err)
	}

	for rows.Next() {
		order := &models.PaymentOrder{}

		err := rows.Scan(
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
			rows.Close()
			return nil, fmt.Errorf("scan payment order: %w", err)
		}

		orders[order.ID] = order
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate payment orders: %w", err)
	}
	rows.Close()

	if len(orders) != len(orderIDs) {
		return nil, models.ErrPaymentOrderNotFound
	}

	return orders, nil
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

func (r *PaymentRepository) CancelOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentOrder, error) {
	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "cancel payment order"),
		slog.String("order_id", orderID.String()),
	)

	var order *models.PaymentOrder
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		var err error
		order, err = cancelOrderTx(ctx, tx, orderID, reasonOrderCanceled)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("cancel payment order: %w", err)
	}

	return order, nil
}

func (r *PaymentRepository) CancelExpiredOrders(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = defaultExpiredOrdersLimit
	}

	var canceledCount int

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const query = `
            SELECT id
            FROM payment_order
            WHERE status IN ('pending', 'processing')
              AND expires_at IS NOT NULL
              AND expires_at <= now()
            ORDER BY expires_at, id
            LIMIT $1
            FOR UPDATE SKIP LOCKED
        `

		rows, err := tx.Query(ctx, query, limit)
		if err != nil {
			return fmt.Errorf("get expired payment orders: %w", err)
		}

		var orderIDs []uuid.UUID

		for rows.Next() {
			var orderID uuid.UUID
			if err := rows.Scan(&orderID); err != nil {
				rows.Close()
				return fmt.Errorf("scan expired order ID: %w", err)
			}

			orderIDs = append(orderIDs, orderID)
		}

		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate expired orders: %w", err)
		}
		rows.Close()

		for _, orderID := range orderIDs {
			if _, err := cancelOrderTx(ctx, tx, orderID, reasonOrderExpired); err != nil {
				return err
			}

			canceledCount++
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("cancel expired payment orders: %w", err)
	}

	return canceledCount, nil
}

func cancelOrderTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, reason string) (*models.PaymentOrder, error) {
	const lockOrder = `
        SELECT 
			id, payer_id, request_key, purpose, origin, expected_amount_minor, currency, status, 
			credited_attempt_id, expires_at, paid_at, canceled_at, created_at, updated_at
        FROM payment_order
        WHERE id = $1
        FOR UPDATE
    `

	order := &models.PaymentOrder{}
	err := tx.QueryRow(ctx, lockOrder, orderID).Scan(
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
		return nil, fmt.Errorf("lock payment order: %w", err)
	}

	switch order.Status {
	case models.StatusCanceled:
		// Идемпотентная повторная отмена
		return order, nil
	case models.StatusPaid, models.StatusFailed:
		return nil, models.ErrForbiddenUpdate
	case models.StatusPending, models.StatusProcessing:
		// Продолжаем отмену
	default:
		return nil, models.ErrPaymentOrderValidation
	}

	const getActiveAttempts = `
        SELECT id
        FROM payment_attempt
        WHERE payment_order_id = $1
          AND status IN ('created', 'processing')
        ORDER BY created_at, id
        FOR UPDATE
    `

	rows, err := tx.Query(ctx, getActiveAttempts, orderID)
	if err != nil {
		return nil, fmt.Errorf("get active payment attempts: %w", err)
	}

	var attemptIDs []uuid.UUID

	for rows.Next() {
		var attemptID uuid.UUID
		if err := rows.Scan(&attemptID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan payment attempt ID: %w", err)
		}

		attemptIDs = append(attemptIDs, attemptID)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate payment attempts: %w", err)
	}
	rows.Close()

	const updateOrder = `
		UPDATE payment_order
		SET status = 'canceled',
			canceled_at = now()
		WHERE id = $1
		AND status IN ('pending', 'processing')
		RETURNING canceled_at, updated_at
	`

	err = tx.QueryRow(ctx, updateOrder, orderID).Scan(
		&order.CanceledAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update payment order status: %w", err)
	}

	order.Status = models.StatusCanceled

	for _, attemptID := range attemptIDs {
		if err := markAttemptCanceled(ctx, tx, orderID, attemptID, reason); err != nil {
			return nil, err
		}
	}

	return order, nil
}

func markAttemptCanceled(ctx context.Context, tx pgx.Tx, orderID, attemptID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return models.ErrPaymentAttemptValidation
	}

	return changeAttemptStatus(ctx, tx, orderID, attemptID, models.AttemptStatusCanceled, reason)
}

func changeAttemptStatus(ctx context.Context, tx pgx.Tx, orderID, attemptID uuid.UUID, targetStatus models.PaymentAttemptStatus, reason string) error {
	var (
		query string
		args  []any
	)

	switch targetStatus {
	case models.AttemptStatusFailed:
		query = `
			UPDATE payment_attempt
			SET status = 'failed', failure_reason = $3
			WHERE id = $1 AND payment_order_id = $2 AND status IN ('created', 'processing')
		`
		args = []any{attemptID, orderID, reason}
	case models.AttemptStatusCanceled:
		query = `
			UPDATE payment_attempt
			SET status = 'canceled', failure_reason = $3
			WHERE id = $1
				AND payment_order_id = $2
				AND status IN ('created', 'processing')
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

func equalOptionalTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	// PostgreSQL timestamptz хранит микросекундную точность.
	return a.Truncate(time.Microsecond).Equal(b.Truncate(time.Microsecond))
}

func uniqueUUIDs(ids []uuid.UUID) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))

	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}

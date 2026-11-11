package paymentrepository

import (
	"context"
	"testing"
	"time"

	"errors"
	"io"
	"log/slog"

	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var (
	testAttemptID = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	testOrderID   = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	testTime      = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	errDB         = errors.New("connection lost")
)

func newTestRepository(t *testing.T) (*PaymentRepository, pgxmock.PgxPoolIface) {
	t.Helper()

	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return &PaymentRepository{pool: pool, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, pool
}

func TestNewPaymentRepository(t *testing.T) {
	repo := NewPaymentRepository(&postgres.DBClient{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	require.NotNil(t, repo)
}

var testPayerID = uuid.MustParse("00000000-0000-4000-8000-000000000003")

var orderColumns = []string{
	"id", "payer_id", "request_key", "purpose", "origin", "expected_amount_minor", "currency", "status",
	"credited_attempt_id", "expires_at", "paid_at", "canceled_at", "created_at", "updated_at",
}

func TestPaymentRepository_GetOrderByID(t *testing.T) {
	expiresAt := testTime.Add(time.Hour)

	tests := []struct {
		name    string
		rows    *pgxmock.Rows
		err     error
		want    *models.PaymentOrder
		wantErr error
	}{
		{
			name: "paid order with all fields",
			rows: pgxmock.NewRows(orderColumns).AddRow(
				testOrderID, testPayerID, "request-key", models.PurposeDonation, models.OriginManual, int64(15000), "RUB", models.StatusPaid,
				&testAttemptID, &expiresAt, &testTime, nil, testTime, testTime,
			),
			want: &models.PaymentOrder{
				ID:                  testOrderID,
				PayerID:             testPayerID,
				RequestKey:          "request-key",
				Purpose:             models.PurposeDonation,
				Origin:              models.OriginManual,
				ExpectedAmountMinor: 15000,
				Currency:            "RUB",
				Status:              models.StatusPaid,
				CreditedAttemptID:   &testAttemptID,
				ExpiresAt:           &expiresAt,
				PaidAt:              &testTime,
				CreatedAt:           testTime,
				UpdatedAt:           testTime,
			},
		},
		{
			name: "pending order without optional fields",
			rows: pgxmock.NewRows(orderColumns).AddRow(
				testOrderID, testPayerID, "request-key", models.PurposeDonation, models.OriginAutoRenewal, int64(500), "RUB", models.StatusPending,
				nil, nil, nil, nil, testTime, testTime,
			),
			want: &models.PaymentOrder{
				ID:                  testOrderID,
				PayerID:             testPayerID,
				RequestKey:          "request-key",
				Purpose:             models.PurposeDonation,
				Origin:              models.OriginAutoRenewal,
				ExpectedAmountMinor: 500,
				Currency:            "RUB",
				Status:              models.StatusPending,
				CreatedAt:           testTime,
				UpdatedAt:           testTime,
			},
		},
		{name: "unknown order", err: pgx.ErrNoRows, wantErr: models.ErrPaymentOrderNotFound},
		{name: "database error", err: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pool := newTestRepository(t)
			expect := pool.ExpectQuery("FROM payment_order").WithArgs(testOrderID)
			if tt.err != nil {
				expect.WillReturnError(tt.err)
			} else {
				expect.WillReturnRows(tt.rows)
			}

			got, err := repo.GetOrderByID(context.Background(), testOrderID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}

const failReason = "insufficient funds"

func expectBeginAndLock(pool pgxmock.PgxPoolIface) {
	pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	pool.ExpectQuery("SELECT status FROM payment_order").WithArgs(testOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"status"}).AddRow(models.StatusPending))
}

func expectAttemptUpdated(pool pgxmock.PgxPoolIface) {
	pool.ExpectExec("UPDATE payment_attempt").WithArgs(testAttemptID, testOrderID, failReason).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
}

func expectHasActive(pool pgxmock.PgxPoolIface, hasActive bool) {
	pool.ExpectQuery("SELECT EXISTS").WithArgs(testOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(hasActive))
}

func TestPaymentRepository_FailAttempt(t *testing.T) {
	tests := []struct {
		name    string
		reason  string
		setup   func(pool pgxmock.PgxPoolIface)
		wantErr error
	}{
		{
			name:   "no active attempts fails order",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				expectAttemptUpdated(pool)
				expectHasActive(pool, false)
				pool.ExpectExec("UPDATE payment_order").WithArgs(testOrderID).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				pool.ExpectCommit()
			},
		},
		{
			name:   "active attempt keeps order open",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				expectAttemptUpdated(pool)
				expectHasActive(pool, true)
				pool.ExpectCommit()
			},
		},
		{
			name:   "order not found",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
				pool.ExpectQuery("SELECT status FROM payment_order").WithArgs(testOrderID).WillReturnError(pgx.ErrNoRows)
				pool.ExpectRollback()
			},
			wantErr: models.ErrPaymentOrderNotFound,
		},
		{
			name:   "lock order error",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
				pool.ExpectQuery("SELECT status FROM payment_order").WithArgs(testOrderID).WillReturnError(errDB)
				pool.ExpectRollback()
			},
			wantErr: errDB,
		},
		{
			name:   "empty reason",
			reason: "   ",
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				pool.ExpectRollback()
			},
			wantErr: models.ErrPaymentAttemptValidation,
		},
		{
			name:   "update attempt error",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				pool.ExpectExec("UPDATE payment_attempt").WithArgs(testAttemptID, testOrderID, failReason).WillReturnError(errDB)
				pool.ExpectRollback()
			},
			wantErr: errDB,
		},
		{
			name:   "attempt not found after empty update",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				pool.ExpectExec("UPDATE payment_attempt").WithArgs(testAttemptID, testOrderID, failReason).
					WillReturnResult(pgxmock.NewResult("UPDATE", 0))
				pool.ExpectQuery("SELECT status, failure_reason FROM payment_attempt").WithArgs(testAttemptID, testOrderID).
					WillReturnError(pgx.ErrNoRows)
				pool.ExpectRollback()
			},
			wantErr: models.ErrPaymentAttemptNotFound,
		},
		{
			name:   "read attempt after empty update error",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				pool.ExpectExec("UPDATE payment_attempt").WithArgs(testAttemptID, testOrderID, failReason).
					WillReturnResult(pgxmock.NewResult("UPDATE", 0))
				pool.ExpectQuery("SELECT status, failure_reason FROM payment_attempt").WithArgs(testAttemptID, testOrderID).
					WillReturnError(errDB)
				pool.ExpectRollback()
			},
			wantErr: errDB,
		},
		{
			name:   "attempt already failed with same reason is idempotent",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				pool.ExpectExec("UPDATE payment_attempt").WithArgs(testAttemptID, testOrderID, failReason).
					WillReturnResult(pgxmock.NewResult("UPDATE", 0))
				reason := failReason
				pool.ExpectQuery("SELECT status, failure_reason FROM payment_attempt").WithArgs(testAttemptID, testOrderID).
					WillReturnRows(pgxmock.NewRows([]string{"status", "failure_reason"}).AddRow(models.AttemptStatusFailed, &reason))
				expectHasActive(pool, true)
				pool.ExpectCommit()
			},
		},
		{
			name:   "attempt already failed with another reason",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				pool.ExpectExec("UPDATE payment_attempt").WithArgs(testAttemptID, testOrderID, failReason).
					WillReturnResult(pgxmock.NewResult("UPDATE", 0))
				reason := "other"
				pool.ExpectQuery("SELECT status, failure_reason FROM payment_attempt").WithArgs(testAttemptID, testOrderID).
					WillReturnRows(pgxmock.NewRows([]string{"status", "failure_reason"}).AddRow(models.AttemptStatusFailed, &reason))
				pool.ExpectRollback()
			},
			wantErr: models.ErrPaymentAttemptValidation,
		},
		{
			name:   "attempt already succeeded",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				pool.ExpectExec("UPDATE payment_attempt").WithArgs(testAttemptID, testOrderID, failReason).
					WillReturnResult(pgxmock.NewResult("UPDATE", 0))
				pool.ExpectQuery("SELECT status, failure_reason FROM payment_attempt").WithArgs(testAttemptID, testOrderID).
					WillReturnRows(pgxmock.NewRows([]string{"status", "failure_reason"}).AddRow(models.AttemptStatusSucceeded, nil))
				pool.ExpectRollback()
			},
			wantErr: models.ErrPaymentAttemptValidation,
		},
		{
			name:   "check active attempts error",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				expectAttemptUpdated(pool)
				pool.ExpectQuery("SELECT EXISTS").WithArgs(testOrderID).WillReturnError(errDB)
				pool.ExpectRollback()
			},
			wantErr: errDB,
		},
		{
			name:   "fail order error",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				expectAttemptUpdated(pool)
				expectHasActive(pool, false)
				pool.ExpectExec("UPDATE payment_order").WithArgs(testOrderID).WillReturnError(errDB)
				pool.ExpectRollback()
			},
			wantErr: errDB,
		},
		{
			name:   "fail order check violation",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				expectAttemptUpdated(pool)
				expectHasActive(pool, false)
				pool.ExpectExec("UPDATE payment_order").WithArgs(testOrderID).WillReturnError(&pgconn.PgError{Code: "23514"})
				pool.ExpectRollback()
			},
			wantErr: models.ErrPaymentOrderValidation,
		},
		{
			name:   "fail order foreign key violation",
			reason: failReason,
			setup: func(pool pgxmock.PgxPoolIface) {
				expectBeginAndLock(pool)
				expectAttemptUpdated(pool)
				expectHasActive(pool, false)
				pool.ExpectExec("UPDATE payment_order").WithArgs(testOrderID).WillReturnError(&pgconn.PgError{Code: "23503"})
				pool.ExpectRollback()
			},
			wantErr: models.ErrPaymentOrderValidation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pool := newTestRepository(t)
			tt.setup(pool)
			// pgx.BeginTxFunc всегда вызывает отложенный Rollback после Commit или после ошибки
			pool.ExpectRollback()

			err := repo.FailAttempt(context.Background(), testOrderID, testAttemptID, tt.reason)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}

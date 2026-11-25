package repository

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var (
	repoTestOrderID         = uuid.MustParse("10000000-0000-4000-8000-000000000001")
	repoTestSecondOrderID   = uuid.MustParse("10000000-0000-4000-8000-000000000002")
	repoTestAttemptID       = uuid.MustParse("20000000-0000-4000-8000-000000000001")
	repoTestSecondAttemptID = uuid.MustParse("20000000-0000-4000-8000-000000000002")
	repoTestCreditedAttempt = uuid.MustParse("20000000-0000-4000-8000-000000000003")
	repoTestPayerID         = uuid.MustParse("30000000-0000-4000-8000-000000000001")
	repoTestAuthorID        = uuid.MustParse("30000000-0000-4000-8000-000000000002")
	repoTestTime            = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	repoTestDBErr           = errors.New("database connection lost")
	repoTestExternalID      = "external-transaction-1"
	repoTestFailureReason   = "insufficient funds"
)

var repoTestOrderColumns = []string{
	"id", "payer_id", "request_key", "purpose", "origin", "expected_amount_minor", "currency", "status",
	"credited_attempt_id", "expires_at", "paid_at", "canceled_at", "created_at", "updated_at",
}

var repoTestAttemptColumns = []string{
	"id", "payment_order_id", "provider", "external_transaction_id", "status",
	"charged_amount_minor", "charged_currency", "failure_reason", "completed_at", "created_at", "updated_at",
}

func newPaymentRepositoryTest(t *testing.T) (*MonetizationRepository, pgxmock.PgxPoolIface) {
	t.Helper()

	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &MonetizationRepository{pool: pool, logger: logger}, pool
}

func repoExpectBegin(pool pgxmock.PgxPoolIface) {
	pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
}

func repoExpectCommit(pool pgxmock.PgxPoolIface) {
	pool.ExpectCommit()
	// pgx.BeginTxFunc always runs its deferred Rollback, including after Commit.
	pool.ExpectRollback()
}

func repoExpectRollback(pool pgxmock.PgxPoolIface) {
	// pgx.BeginTxFunc rolls back in the callback-error branch and then calls
	// Rollback once more from its deferred cleanup. pgxmock v5 expects both calls.
	pool.ExpectRollback().Times(2)
}

func repoExpectedOrder(req dto.CreateDonationWithPayment, expiresAt *time.Time) *models.PaymentOrder {
	return &models.PaymentOrder{
		ID:                  repoTestOrderID,
		PayerID:             req.DonorID,
		RequestKey:          req.RequestKey,
		Purpose:             models.PurposeDonation,
		Origin:              models.OriginManual,
		ExpectedAmountMinor: req.AmountMinor,
		Currency:            req.Currency,
		Status:              models.StatusPending,
		CreditedAttemptID:   nil,
		ExpiresAt:           expiresAt,
		PaidAt:              nil,
		CanceledAt:          nil,
		CreatedAt:           repoTestTime,
		UpdatedAt:           repoTestTime,
	}
}

func repoOrderRow(id uuid.UUID, status models.PaymentOrderStatus, creditedAttemptID *uuid.UUID) *pgxmock.Rows {
	return pgxmock.NewRows(repoTestOrderColumns).AddRow(
		id,
		repoTestPayerID,
		"request-key-"+id.String(),
		models.PurposeDonation,
		models.OriginManual,
		int64(15000),
		"RUB",
		status,
		creditedAttemptID,
		nil,
		nil,
		nil,
		repoTestTime,
		repoTestTime,
	)
}

func repoOrderRowForRequest(req dto.CreateDonationWithPayment, expiresAt *time.Time) *pgxmock.Rows {
	return pgxmock.NewRows(repoTestOrderColumns).AddRow(
		repoTestOrderID,
		req.DonorID,
		req.RequestKey,
		models.PurposeDonation,
		models.OriginManual,
		req.AmountMinor,
		req.Currency,
		models.StatusPending,
		nil,
		expiresAt,
		nil,
		nil,
		repoTestTime,
		repoTestTime,
	)
}

func repoAttemptRow(id, orderID uuid.UUID, status models.PaymentAttemptStatus) *pgxmock.Rows {
	return pgxmock.NewRows(repoTestAttemptColumns).AddRow(
		id,
		orderID,
		defaultPaymentProvider,
		nil,
		status,
		nil,
		nil,
		nil,
		nil,
		repoTestTime,
		repoTestTime,
	)
}

func repoExpectStartAttemptLocks(
	pool pgxmock.PgxPoolIface,
	orderStatus models.PaymentOrderStatus,
	attemptStatus models.PaymentAttemptStatus,
) {
	repoExpectBegin(pool)
	pool.ExpectQuery("FROM payment_order").WithArgs(repoTestOrderID).
		WillReturnRows(repoOrderRow(repoTestOrderID, orderStatus, nil))
	pool.ExpectQuery("FROM payment_attempt").WithArgs(repoTestOrderID).
		WillReturnRows(repoAttemptRow(repoTestAttemptID, repoTestOrderID, attemptStatus))
}

func TestMonetizationRepository_New(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := NewMonetizationRepository(&postgres.DBClient{}, logger)
	require.NotNil(t, repo)
	require.Same(t, logger, repo.logger)
}

func TestMonetizationRepository_CreateDonationWithPayment_Atomic(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	expiresAt := repoTestTime.Add(time.Minute * 15)
	message := "thank you"
	req := dto.CreateDonationWithPayment{
		DonorID:     repoTestPayerID,
		AuthorID:    repoTestAuthorID,
		RequestKey:  "atomic-donation-key",
		Message:     &message,
		IsAnonymous: true,
		AmountMinor: 15000,
		Currency:    "RUB",
		ExpiresAt:   &expiresAt,
	}
	repoExpectBegin(pool)
	pool.ExpectQuery("INSERT INTO payment_order").WithArgs(
		req.DonorID, req.RequestKey, models.PurposeDonation, models.OriginManual,
		req.AmountMinor, req.Currency, req.ExpiresAt,
	).WillReturnRows(repoOrderRowForRequest(req, req.ExpiresAt))
	pool.ExpectExec("INSERT INTO payment_attempt").WithArgs(repoTestOrderID, defaultPaymentProvider).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	pool.ExpectQuery("INSERT INTO donation").WithArgs(
		repoTestOrderID, req.DonorID, req.AuthorID, req.Message, req.IsAnonymous,
	).WillReturnRows(pgxmock.NewRows([]string{"created_at", "updated_at"}).AddRow(repoTestTime, repoTestTime))
	repoExpectCommit(pool)

	donation, order, err := repo.CreateDonationWithPayment(context.Background(), req)

	require.NoError(t, err)
	require.Equal(t, repoTestOrderID, donation.PaymentOrderID)
	require.Equal(t, req.DonorID, donation.DonorID)
	require.Equal(t, req.AuthorID, donation.AuthorID)
	require.Equal(t, req.Message, donation.Message)
	require.True(t, donation.IsAnonymous)
	require.Equal(t, repoTestTime, donation.CreatedAt)
	require.Equal(t, repoExpectedOrder(req, req.ExpiresAt), order)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_CreateDonationWithPayment_RollsBackOnDonationFailure(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	req := dto.CreateDonationWithPayment{
		DonorID: repoTestPayerID, AuthorID: repoTestAuthorID, RequestKey: "atomic-donation-key",
		AmountMinor: 15000, Currency: "RUB",
	}
	repoExpectBegin(pool)
	pool.ExpectQuery("INSERT INTO payment_order").WithArgs(
		req.DonorID, req.RequestKey, models.PurposeDonation, models.OriginManual,
		req.AmountMinor, req.Currency, (*time.Time)(nil),
	).WillReturnRows(repoOrderRowForRequest(req, nil))
	pool.ExpectExec("INSERT INTO payment_attempt").WithArgs(repoTestOrderID, defaultPaymentProvider).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	pool.ExpectQuery("INSERT INTO donation").WithArgs(
		repoTestOrderID, req.DonorID, req.AuthorID, req.Message, req.IsAnonymous,
	).WillReturnError(&pgconn.PgError{Code: "23503"})
	repoExpectRollback(pool)

	donation, order, err := repo.CreateDonationWithPayment(context.Background(), req)

	require.Nil(t, donation)
	require.Nil(t, order)
	require.ErrorIs(t, err, models.ErrDonationValidation)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_CreateDonationWithPayment_IdempotentRetry(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	message := "thank you"
	req := dto.CreateDonationWithPayment{
		DonorID: repoTestPayerID, AuthorID: repoTestAuthorID, RequestKey: "atomic-donation-key",
		Message: &message, IsAnonymous: true, AmountMinor: 15000, Currency: "RUB",
	}
	repoExpectBegin(pool)
	pool.ExpectQuery("INSERT INTO payment_order").WithArgs(
		req.DonorID, req.RequestKey, models.PurposeDonation, models.OriginManual,
		req.AmountMinor, req.Currency, (*time.Time)(nil),
	).WillReturnError(pgx.ErrNoRows)
	pool.ExpectQuery("FROM payment_order").WithArgs(req.DonorID, req.RequestKey).
		WillReturnRows(repoOrderRowForRequest(req, nil))
	pool.ExpectQuery("FROM payment_attempt").WithArgs(repoTestOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(repoTestAttemptID))
	pool.ExpectQuery("INSERT INTO donation").WithArgs(
		repoTestOrderID, req.DonorID, req.AuthorID, req.Message, req.IsAnonymous,
	).WillReturnError(pgx.ErrNoRows)
	pool.ExpectQuery("FROM donation").WithArgs(repoTestOrderID).
		WillReturnRows(pgxmock.NewRows([]string{
			"donor_id", "author_id", "message", "is_anonymous", "created_at", "updated_at",
		}).AddRow(req.DonorID, req.AuthorID, &message, req.IsAnonymous, repoTestTime, repoTestTime))
	repoExpectCommit(pool)

	donation, order, err := repo.CreateDonationWithPayment(context.Background(), req)

	require.NoError(t, err)
	require.Equal(t, repoTestOrderID, donation.PaymentOrderID)
	require.Equal(t, req.DonorID, donation.DonorID)
	require.Equal(t, req.AuthorID, donation.AuthorID)
	require.Equal(t, req.Message, donation.Message)
	require.True(t, donation.IsAnonymous)
	require.Equal(t, repoExpectedOrder(req, nil), order)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestPaymentRepositoryFull_StartAttempt(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "starts pending order and created attempt", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectStartAttemptLocks(pool, models.StatusPending, models.AttemptStatusCreated)
			pool.ExpectExec("UPDATE payment_order").WithArgs(repoTestOrderID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			repoExpectCommit(pool)

			order, attempt, started, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.NoError(t, err)
			require.True(t, started)
			require.Equal(t, models.StatusProcessing, order.Status)
			require.Equal(t, models.AttemptStatusProcessing, attempt.Status)
			require.Equal(t, repoTestOrderID, order.ID)
			require.Equal(t, repoTestAttemptID, attempt.ID)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "does not start attempt when order is not pending", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectStartAttemptLocks(pool, models.StatusPaid, models.AttemptStatusCreated)
			repoExpectCommit(pool)

			order, attempt, started, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.NoError(t, err)
			require.False(t, started)
			require.Equal(t, models.StatusPaid, order.Status)
			require.Equal(t, models.AttemptStatusCreated, attempt.Status)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "does not start attempt when latest attempt is already processing", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectStartAttemptLocks(pool, models.StatusPending, models.AttemptStatusProcessing)
			repoExpectCommit(pool)

			order, attempt, started, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.NoError(t, err)
			require.False(t, started)
			require.Equal(t, models.StatusPending, order.Status)
			require.Equal(t, models.AttemptStatusProcessing, attempt.Status)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "does not start attempt when order has expired", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectStartAttemptLocks(pool, models.StatusPending, models.AttemptStatusCreated)
			pool.ExpectExec("UPDATE payment_order").WithArgs(repoTestOrderID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			repoExpectCommit(pool)

			order, attempt, started, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.NoError(t, err)
			require.False(t, started)
			require.Equal(t, models.StatusPending, order.Status)
			require.Equal(t, models.AttemptStatusCreated, attempt.Status)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "order not found", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("FROM payment_order").WithArgs(repoTestOrderID).WillReturnError(pgx.ErrNoRows)
			repoExpectRollback(pool)

			order, attempt, started, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.ErrorIs(t, err, models.ErrPaymentOrderNotFound)
			require.Nil(t, order)
			require.Nil(t, attempt)
			require.False(t, started)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps order lock error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("FROM payment_order").WithArgs(repoTestOrderID).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			_, _, started, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.ErrorIs(t, err, repoTestDBErr)
			require.False(t, started)
			require.Contains(t, err.Error(), "start payment attempt")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "attempt not found", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("FROM payment_order").WithArgs(repoTestOrderID).
				WillReturnRows(repoOrderRow(repoTestOrderID, models.StatusPending, nil))
			pool.ExpectQuery("FROM payment_attempt").WithArgs(repoTestOrderID).WillReturnError(pgx.ErrNoRows)
			repoExpectRollback(pool)

			_, _, started, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.ErrorIs(t, err, models.ErrPaymentAttemptNotFound)
			require.False(t, started)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps attempt lock error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("FROM payment_order").WithArgs(repoTestOrderID).
				WillReturnRows(repoOrderRow(repoTestOrderID, models.StatusPending, nil))
			pool.ExpectQuery("FROM payment_attempt").WithArgs(repoTestOrderID).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			_, _, _, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "lock payment attempt")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps order update error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectStartAttemptLocks(pool, models.StatusPending, models.AttemptStatusCreated)
			pool.ExpectExec("UPDATE payment_order").WithArgs(repoTestOrderID).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			_, _, _, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "mark payment order processing")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "rolls back if attempt transition affects no rows", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectStartAttemptLocks(pool, models.StatusPending, models.AttemptStatusCreated)
			pool.ExpectExec("UPDATE payment_order").WithArgs(repoTestOrderID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			repoExpectRollback(pool)

			_, _, started, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.ErrorIs(t, err, models.ErrPaymentAttemptValidation)
			require.False(t, started)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps attempt update error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectStartAttemptLocks(pool, models.StatusPending, models.AttemptStatusCreated)
			pool.ExpectExec("UPDATE payment_order").WithArgs(repoTestOrderID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			_, _, _, err := repo.StartAttempt(context.Background(), repoTestOrderID)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "mark payment attempt processing")
			require.NoError(t, pool.ExpectationsWereMet())
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestPaymentRepositoryFull_GetOrdersByIDs(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "empty input returns empty map without querying", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			got, err := repo.GetOrdersByIDs(context.Background(), nil)
			require.NoError(t, err)
			require.Empty(t, got)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "deduplicates IDs and returns all orders", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			ids := []uuid.UUID{repoTestOrderID, repoTestSecondOrderID, repoTestOrderID}
			pool.ExpectQuery("FROM payment_order").WithArgs([]uuid.UUID{repoTestOrderID, repoTestSecondOrderID}).
				WillReturnRows(repoOrderRow(repoTestOrderID, models.StatusPending, nil).
					AddRow(repoTestSecondOrderID, repoTestPayerID, "request-key-2", models.PurposeDonation,
						models.OriginManual, int64(15000), "RUB", models.StatusProcessing,
						nil, nil, nil, nil, repoTestTime, repoTestTime))

			got, err := repo.GetOrdersByIDs(context.Background(), ids)

			require.NoError(t, err)
			require.Len(t, got, 2)
			require.Equal(t, models.StatusPending, got[repoTestOrderID].Status)
			require.Equal(t, models.StatusProcessing, got[repoTestSecondOrderID].Status)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "returns not found when at least one order is absent", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			pool.ExpectQuery("FROM payment_order").WithArgs([]uuid.UUID{repoTestOrderID, repoTestSecondOrderID}).
				WillReturnRows(repoOrderRow(repoTestOrderID, models.StatusPending, nil))

			got, err := repo.GetOrdersByIDs(context.Background(), []uuid.UUID{repoTestOrderID, repoTestSecondOrderID})

			require.ErrorIs(t, err, models.ErrPaymentOrderNotFound)
			require.Nil(t, got)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps query error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			pool.ExpectQuery("FROM payment_order").WithArgs([]uuid.UUID{repoTestOrderID}).WillReturnError(repoTestDBErr)

			got, err := repo.GetOrdersByIDs(context.Background(), []uuid.UUID{repoTestOrderID})

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "get payment orders")
			require.Nil(t, got)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "returns scan error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			pool.ExpectQuery("FROM payment_order").WithArgs([]uuid.UUID{repoTestOrderID}).
				WillReturnRows(pgxmock.NewRows(repoTestOrderColumns).AddRow(
					"not-a-uuid", repoTestPayerID, "request-key", models.PurposeDonation, models.OriginManual,
					int64(15000), "RUB", models.StatusPending, nil, nil, nil, nil, repoTestTime, repoTestTime,
				))

			got, err := repo.GetOrdersByIDs(context.Background(), []uuid.UUID{repoTestOrderID})

			require.Error(t, err)
			require.Contains(t, err.Error(), "scan payment order")
			require.Nil(t, got)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "returns row iteration error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			rows := repoOrderRow(repoTestOrderID, models.StatusPending, nil).CloseError(repoTestDBErr)
			pool.ExpectQuery("FROM payment_order").WithArgs([]uuid.UUID{repoTestOrderID}).WillReturnRows(rows)

			got, err := repo.GetOrdersByIDs(context.Background(), []uuid.UUID{repoTestOrderID})

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "iterate payment orders")
			require.Nil(t, got)
			require.NoError(t, pool.ExpectationsWereMet())
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestPaymentRepositoryFull_GetOrderByID(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "returns order", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			pool.ExpectQuery("FROM payment_order WHERE id = \\$1").WithArgs(repoTestOrderID).
				WillReturnRows(repoOrderRow(repoTestOrderID, models.StatusPending, nil))

			got, err := repo.GetOrderByID(context.Background(), repoTestOrderID)

			require.NoError(t, err)
			require.Equal(t, repoTestOrderID, got.ID)
			require.Equal(t, models.StatusPending, got.Status)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "maps no rows to not found", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			pool.ExpectQuery("FROM payment_order WHERE id = \\$1").WithArgs(repoTestOrderID).WillReturnError(pgx.ErrNoRows)

			got, err := repo.GetOrderByID(context.Background(), repoTestOrderID)

			require.ErrorIs(t, err, models.ErrPaymentOrderNotFound)
			require.Nil(t, got)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps database error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			pool.ExpectQuery("FROM payment_order WHERE id = \\$1").WithArgs(repoTestOrderID).WillReturnError(repoTestDBErr)

			got, err := repo.GetOrderByID(context.Background(), repoTestOrderID)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "get payment order error")
			require.Nil(t, got)
			require.NoError(t, pool.ExpectationsWereMet())
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func repoCompletionInput() dto.AttemptToComplete {
	return dto.AttemptToComplete{
		ID:                    repoTestAttemptID,
		OrderID:               repoTestOrderID,
		ExternalTransactionID: repoTestExternalID,
		ChargedAmountMinor:    15000,
		ChargedCurrency:       "RUB",
		CompletedAt:           repoTestTime,
	}
}

func repoExpectCompletionLocks(
	pool pgxmock.PgxPoolIface,
	orderStatus models.PaymentOrderStatus,
	expectedAmount int64,
	expectedCurrency string,
	creditedAttemptID *uuid.UUID,
	attemptOrderID uuid.UUID,
	attemptStatus models.PaymentAttemptStatus,
	externalID *string,
	chargedAmount *int64,
	chargedCurrency *string,
) {
	repoExpectBegin(pool)
	pool.ExpectQuery("SELECT status, expected_amount_minor, currency, credited_attempt_id").WithArgs(repoTestOrderID).
		WillReturnRows(pgxmock.NewRows([]string{
			"status", "expected_amount_minor", "currency", "credited_attempt_id",
		}).AddRow(orderStatus, expectedAmount, expectedCurrency, creditedAttemptID))
	pool.ExpectQuery("SELECT payment_order_id, status, external_transaction_id").WithArgs(repoTestAttemptID).
		WillReturnRows(pgxmock.NewRows([]string{
			"payment_order_id", "status", "external_transaction_id", "charged_amount_minor", "charged_currency",
		}).AddRow(attemptOrderID, attemptStatus, externalID, chargedAmount, chargedCurrency))
}

func repoExpectSaveSuccessfulAttempt(pool pgxmock.PgxPoolIface, input dto.AttemptToComplete) {
	pool.ExpectExec("UPDATE payment_attempt").WithArgs(
		input.ID, input.OrderID, input.ExternalTransactionID,
		input.ChargedAmountMinor, input.ChargedCurrency, input.CompletedAt,
	).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
}

func TestPaymentRepositoryFull_CompleteSuccessfulAttempt(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "marks attempt and order paid", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.StatusProcessing, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("UPDATE payment_order").WithArgs(
				input.OrderID, input.ID, input.ChargedAmountMinor, input.ChargedCurrency,
			).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			repoExpectCommit(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.NoError(t, err)
			require.False(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "accepts already succeeded credited attempt idempotently", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			externalID, amount, currency := input.ExternalTransactionID, input.ChargedAmountMinor, input.ChargedCurrency
			repoExpectCompletionLocks(pool, models.StatusPaid, 15000, "RUB", &repoTestAttemptID,
				repoTestOrderID, models.AttemptStatusSucceeded, &externalID, &amount, &currency)
			repoExpectCommit(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.NoError(t, err)
			require.False(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "rejects succeeded attempt with different provider transaction", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			externalID, amount, currency := "another-external-id", input.ChargedAmountMinor, input.ChargedCurrency
			repoExpectCompletionLocks(pool, models.StatusPaid, 15000, "RUB", &repoTestAttemptID,
				repoTestOrderID, models.AttemptStatusSucceeded, &externalID, &amount, &currency)
			repoExpectRollback(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, models.ErrPaymentAttemptValidation)
			require.False(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "rejects succeeded attempt with different charged amount", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			externalID, amount, currency := input.ExternalTransactionID, int64(14999), input.ChargedCurrency
			repoExpectCompletionLocks(pool, models.StatusPaid, 15000, "RUB", &repoTestAttemptID,
				repoTestOrderID, models.AttemptStatusSucceeded, &externalID, &amount, &currency)
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, models.ErrPaymentAttemptValidation)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "rejects succeeded attempt belonging to another order", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			externalID, amount, currency := input.ExternalTransactionID, input.ChargedAmountMinor, input.ChargedCurrency
			repoExpectCompletionLocks(pool, models.StatusProcessing, 15000, "RUB", nil,
				repoTestSecondOrderID, models.AttemptStatusSucceeded, &externalID, &amount, &currency)
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, models.ErrPaymentAttemptValidation)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "creates refund for late successful response after cancellation", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.StatusCanceled, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusCanceled, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("INSERT INTO refund").WithArgs(repoTestAttemptID, input.ChargedAmountMinor, reasonOrderCanceled).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			repoExpectCommit(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.NoError(t, err)
			require.True(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "creates refund for duplicate successful charge on paid order", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.StatusPaid, 15000, "RUB", &repoTestCreditedAttempt,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("INSERT INTO refund").WithArgs(repoTestAttemptID, input.ChargedAmountMinor, reasonDuplicatePayment).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			repoExpectCommit(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.NoError(t, err)
			require.True(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "does not request duplicate refund when paid order was credited by same attempt", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			externalID, amount, currency := input.ExternalTransactionID, input.ChargedAmountMinor, input.ChargedCurrency
			repoExpectCompletionLocks(pool, models.StatusPaid, 15000, "RUB", &repoTestAttemptID,
				repoTestOrderID, models.AttemptStatusSucceeded, &externalID, &amount, &currency)
			repoExpectCommit(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.NoError(t, err)
			require.False(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "refunds amount mismatch after marking order failed", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			input.ChargedAmountMinor = 14000
			repoExpectCompletionLocks(pool, models.StatusProcessing, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("UPDATE payment_order SET status = 'failed'").WithArgs(repoTestOrderID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			pool.ExpectExec("INSERT INTO refund").WithArgs(repoTestAttemptID, input.ChargedAmountMinor, reasonAmountMismatch).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			repoExpectCommit(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.NoError(t, err)
			require.True(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "refunds currency mismatch after marking order failed", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			input.ChargedCurrency = "USD"
			repoExpectCompletionLocks(pool, models.StatusPending, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("UPDATE payment_order SET status = 'failed'").WithArgs(repoTestOrderID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			pool.ExpectExec("INSERT INTO refund").WithArgs(repoTestAttemptID, input.ChargedAmountMinor, reasonAmountMismatch).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			repoExpectCommit(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.NoError(t, err)
			require.True(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "can recover a failed order after a successful retry", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.StatusFailed, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusFailed, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("UPDATE payment_order").WithArgs(
				repoTestOrderID, repoTestAttemptID, input.ChargedAmountMinor, input.ChargedCurrency,
			).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			repoExpectCommit(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.NoError(t, err)
			require.False(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "rejects unsupported order status", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.PaymentOrderStatus("unknown"), 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, models.ErrPaymentOrderValidation)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "maps missing order", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT status, expected_amount_minor, currency, credited_attempt_id").
				WithArgs(repoTestOrderID).WillReturnError(pgx.ErrNoRows)
			repoExpectRollback(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), repoCompletionInput())

			require.ErrorIs(t, err, models.ErrPaymentOrderNotFound)
			require.False(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps order lock error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT status, expected_amount_minor, currency, credited_attempt_id").
				WithArgs(repoTestOrderID).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), repoCompletionInput())

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "lock payment order")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "maps missing attempt", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT status, expected_amount_minor, currency, credited_attempt_id").
				WithArgs(repoTestOrderID).WillReturnRows(pgxmock.NewRows([]string{
				"status", "expected_amount_minor", "currency", "credited_attempt_id",
			}).AddRow(models.StatusProcessing, int64(15000), "RUB", nil))
			pool.ExpectQuery("SELECT payment_order_id, status, external_transaction_id").
				WithArgs(repoTestAttemptID).WillReturnError(pgx.ErrNoRows)
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), repoCompletionInput())

			require.ErrorIs(t, err, models.ErrPaymentAttemptNotFound)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps attempt lock error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT status, expected_amount_minor, currency, credited_attempt_id").
				WithArgs(repoTestOrderID).WillReturnRows(pgxmock.NewRows([]string{
				"status", "expected_amount_minor", "currency", "credited_attempt_id",
			}).AddRow(models.StatusProcessing, int64(15000), "RUB", nil))
			pool.ExpectQuery("SELECT payment_order_id, status, external_transaction_id").
				WithArgs(repoTestAttemptID).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), repoCompletionInput())

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "lock payment attempt")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "maps attempt constraint error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.StatusProcessing, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(
				input.ID, input.OrderID, input.ExternalTransactionID,
				input.ChargedAmountMinor, input.ChargedCurrency, input.CompletedAt,
			).WillReturnError(&pgconn.PgError{Code: "23514"})
			repoExpectRollback(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, models.ErrPaymentAttemptValidation)
			require.False(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "maps zero rows when saving successful attempt", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.StatusProcessing, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(
				input.ID, input.OrderID, input.ExternalTransactionID,
				input.ChargedAmountMinor, input.ChargedCurrency, input.CompletedAt,
			).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, models.ErrPaymentAttemptValidation)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps generic successful attempt update error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.StatusProcessing, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(
				input.ID, input.OrderID, input.ExternalTransactionID,
				input.ChargedAmountMinor, input.ChargedCurrency, input.CompletedAt,
			).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "mark payment attempt succeeded")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "maps order constraint error when marking paid", run: func(t *testing.T) {
			for _, code := range []string{"23503", "23514", "23505"} {
				t.Run(code, func(t *testing.T) {
					repo, pool := newPaymentRepositoryTest(t)
					input := repoCompletionInput()
					repoExpectCompletionLocks(pool, models.StatusProcessing, 15000, "RUB", nil,
						repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
					repoExpectSaveSuccessfulAttempt(pool, input)
					pool.ExpectExec("UPDATE payment_order").WithArgs(
						input.OrderID, input.ID, input.ChargedAmountMinor, input.ChargedCurrency,
					).WillReturnError(&pgconn.PgError{Code: code})
					repoExpectRollback(pool)

					_, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

					require.ErrorIs(t, err, models.ErrPaymentOrderValidation)
					require.NoError(t, pool.ExpectationsWereMet())
				})
			}
		}},

		{name: "maps zero rows when marking order paid", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.StatusProcessing, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("UPDATE payment_order").WithArgs(
				input.OrderID, input.ID, input.ChargedAmountMinor, input.ChargedCurrency,
			).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, models.ErrPaymentOrderValidation)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "maps fail-order constraint violation in amount mismatch path", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			input.ChargedAmountMinor--
			repoExpectCompletionLocks(pool, models.StatusProcessing, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("UPDATE payment_order SET status = 'failed'").WithArgs(repoTestOrderID).
				WillReturnError(&pgconn.PgError{Code: "23514"})
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, models.ErrPaymentOrderValidation)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps fail-order database error in amount mismatch path", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			input.ChargedAmountMinor--
			repoExpectCompletionLocks(pool, models.StatusProcessing, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusProcessing, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("UPDATE payment_order SET status = 'failed'").WithArgs(repoTestOrderID).
				WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			_, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "mark payment order failed")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps refund insert error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.StatusCanceled, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusCanceled, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("INSERT INTO refund").WithArgs(repoTestAttemptID, input.ChargedAmountMinor, reasonOrderCanceled).
				WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.ErrorIs(t, err, repoTestDBErr)
			require.False(t, refund)
			require.Contains(t, err.Error(), "insert refund")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "is idempotent when refund already exists", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			input := repoCompletionInput()
			repoExpectCompletionLocks(pool, models.StatusCanceled, 15000, "RUB", nil,
				repoTestOrderID, models.AttemptStatusCanceled, nil, nil, nil)
			repoExpectSaveSuccessfulAttempt(pool, input)
			pool.ExpectExec("INSERT INTO refund").WithArgs(repoTestAttemptID, input.ChargedAmountMinor, reasonOrderCanceled).
				WillReturnResult(pgxmock.NewResult("INSERT", 0))
			repoExpectCommit(pool)

			refund, err := repo.CompleteSuccessfulAttempt(context.Background(), input)

			require.NoError(t, err)
			require.True(t, refund)
			require.NoError(t, pool.ExpectationsWereMet())
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func repoExpectFailAttemptOrderLock(pool pgxmock.PgxPoolIface, status models.PaymentOrderStatus) {
	repoExpectBegin(pool)
	pool.ExpectQuery("SELECT status FROM payment_order").WithArgs(repoTestOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"status"}).AddRow(status))
}

func repoExpectFailAttemptUpdate(pool pgxmock.PgxPoolIface, reason string) {
	pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID, reason).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
}

func repoExpectHasActiveAttempts(pool pgxmock.PgxPoolIface, active bool) {
	pool.ExpectQuery("SELECT EXISTS").WithArgs(repoTestOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(active))
}

func TestPaymentRepositoryFull_FailAttempt(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "marks attempt failed and fails order when no active attempts remain", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			repoExpectFailAttemptUpdate(pool, repoTestFailureReason)
			repoExpectHasActiveAttempts(pool, false)
			pool.ExpectExec("UPDATE payment_order SET status = 'failed'").WithArgs(repoTestOrderID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			repoExpectCommit(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.NoError(t, err)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "keeps order active when another attempt is still active", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			repoExpectFailAttemptUpdate(pool, repoTestFailureReason)
			repoExpectHasActiveAttempts(pool, true)
			repoExpectCommit(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.NoError(t, err)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "trims failure reason before update", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			repoExpectFailAttemptUpdate(pool, repoTestFailureReason)
			repoExpectHasActiveAttempts(pool, true)
			repoExpectCommit(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, "  "+repoTestFailureReason+"  ")

			require.NoError(t, err)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "maps missing order", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT status FROM payment_order").WithArgs(repoTestOrderID).WillReturnError(pgx.ErrNoRows)
			repoExpectRollback(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.ErrorIs(t, err, models.ErrPaymentOrderNotFound)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps order lock error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT status FROM payment_order").WithArgs(repoTestOrderID).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "lock payment order")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "rejects empty reason", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			repoExpectRollback(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, "  \t ")

			require.ErrorIs(t, err, models.ErrPaymentAttemptValidation)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps attempt update error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID, repoTestFailureReason).
				WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "change payment attempt status")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "maps attempt not found after zero-row update", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID, repoTestFailureReason).
				WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			pool.ExpectQuery("SELECT status, failure_reason FROM payment_attempt").
				WithArgs(repoTestAttemptID, repoTestOrderID).WillReturnError(pgx.ErrNoRows)
			repoExpectRollback(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.ErrorIs(t, err, models.ErrPaymentAttemptNotFound)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps fallback attempt query error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID, repoTestFailureReason).
				WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			pool.ExpectQuery("SELECT status, failure_reason FROM payment_attempt").
				WithArgs(repoTestAttemptID, repoTestOrderID).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "get payment attempt after status update")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "is idempotent when attempt already failed for same reason", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID, repoTestFailureReason).
				WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			reason := repoTestFailureReason
			pool.ExpectQuery("SELECT status, failure_reason FROM payment_attempt").
				WithArgs(repoTestAttemptID, repoTestOrderID).
				WillReturnRows(pgxmock.NewRows([]string{"status", "failure_reason"}).AddRow(models.AttemptStatusFailed, &reason))
			repoExpectHasActiveAttempts(pool, true)
			repoExpectCommit(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.NoError(t, err)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "rejects already failed attempt with another reason", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID, repoTestFailureReason).
				WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			otherReason := "another reason"
			pool.ExpectQuery("SELECT status, failure_reason FROM payment_attempt").
				WithArgs(repoTestAttemptID, repoTestOrderID).
				WillReturnRows(pgxmock.NewRows([]string{"status", "failure_reason"}).AddRow(models.AttemptStatusFailed, &otherReason))
			repoExpectRollback(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.ErrorIs(t, err, models.ErrPaymentAttemptValidation)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "rejects attempt that is already succeeded", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID, repoTestFailureReason).
				WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			pool.ExpectQuery("SELECT status, failure_reason FROM payment_attempt").
				WithArgs(repoTestAttemptID, repoTestOrderID).
				WillReturnRows(pgxmock.NewRows([]string{"status", "failure_reason"}).AddRow(models.AttemptStatusSucceeded, nil))
			repoExpectRollback(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.ErrorIs(t, err, models.ErrPaymentAttemptValidation)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps active attempt check error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			repoExpectFailAttemptUpdate(pool, repoTestFailureReason)
			pool.ExpectQuery("SELECT EXISTS").WithArgs(repoTestOrderID).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "check active attempts")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps fail order error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
			repoExpectFailAttemptUpdate(pool, repoTestFailureReason)
			repoExpectHasActiveAttempts(pool, false)
			pool.ExpectExec("UPDATE payment_order SET status = 'failed'").WithArgs(repoTestOrderID).
				WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Contains(t, err.Error(), "mark payment order failed")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "maps fail order constraint violation", run: func(t *testing.T) {
			for _, code := range []string{"23503", "23514"} {
				t.Run(code, func(t *testing.T) {
					repo, pool := newPaymentRepositoryTest(t)
					repoExpectFailAttemptOrderLock(pool, models.StatusProcessing)
					repoExpectFailAttemptUpdate(pool, repoTestFailureReason)
					repoExpectHasActiveAttempts(pool, false)
					pool.ExpectExec("UPDATE payment_order SET status = 'failed'").WithArgs(repoTestOrderID).
						WillReturnError(&pgconn.PgError{Code: code})
					repoExpectRollback(pool)

					err := repo.FailAttempt(context.Background(), repoTestOrderID, repoTestAttemptID, repoTestFailureReason)

					require.ErrorIs(t, err, models.ErrPaymentOrderValidation)
					require.NoError(t, pool.ExpectationsWereMet())
				})
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func repoExpectExpiredOrderCancellation(pool pgxmock.PgxPoolIface, orderID, attemptID uuid.UUID, status models.PaymentOrderStatus) {
	pool.ExpectQuery("FROM payment_order").WithArgs(orderID).
		WillReturnRows(pgxmock.NewRows(repoTestOrderColumns).AddRow(
			orderID, repoTestPayerID, "request-key-"+orderID.String(), models.PurposeDonation,
			models.OriginManual, int64(15000), "RUB", status, nil, nil, nil, nil, repoTestTime, repoTestTime,
		))
	rows := pgxmock.NewRows([]string{"id"})
	if attemptID != uuid.Nil {
		rows.AddRow(attemptID)
	}
	pool.ExpectQuery("FROM payment_attempt").WithArgs(orderID).WillReturnRows(rows)
	pool.ExpectQuery("UPDATE payment_order").WithArgs(orderID).
		WillReturnRows(pgxmock.NewRows([]string{"canceled_at", "updated_at"}).AddRow(&repoTestTime, repoTestTime))
	if attemptID != uuid.Nil {
		pool.ExpectExec("UPDATE payment_attempt").WithArgs(attemptID, orderID, reasonOrderExpired).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	}
}

func TestPaymentRepositoryFull_CancelExpiredOrders(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "uses provided limit and cancels selected expired orders", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT id\\s+FROM payment_order").WithArgs(2).
				WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(repoTestOrderID).AddRow(repoTestSecondOrderID))
			repoExpectExpiredOrderCancellation(pool, repoTestOrderID, repoTestAttemptID, models.StatusPending)
			repoExpectExpiredOrderCancellation(pool, repoTestSecondOrderID, uuid.Nil, models.StatusProcessing)
			repoExpectCommit(pool)

			count, err := repo.CancelExpiredOrders(context.Background(), 2)

			require.NoError(t, err)
			require.Equal(t, 2, count)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "uses default limit for zero or negative values", run: func(t *testing.T) {
			for _, limit := range []int{0, -1} {
				name := "negative"
				if limit == 0 {
					name = "zero"
				}
				t.Run(name, func(t *testing.T) {
					repo, pool := newPaymentRepositoryTest(t)
					repoExpectBegin(pool)
					pool.ExpectQuery("SELECT id\\s+FROM payment_order").WithArgs(defaultExpiredOrdersLimit).
						WillReturnRows(pgxmock.NewRows([]string{"id"}))
					repoExpectCommit(pool)

					count, err := repo.CancelExpiredOrders(context.Background(), limit)

					require.NoError(t, err)
					require.Zero(t, count)
					require.NoError(t, pool.ExpectationsWereMet())
				})
			}
		}},

		{name: "returns zero when there are no expired orders", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT id\\s+FROM payment_order").WithArgs(10).
				WillReturnRows(pgxmock.NewRows([]string{"id"}))
			repoExpectCommit(pool)

			count, err := repo.CancelExpiredOrders(context.Background(), 10)

			require.NoError(t, err)
			require.Zero(t, count)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "wraps expired order selection error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT id\\s+FROM payment_order").WithArgs(10).WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			count, err := repo.CancelExpiredOrders(context.Background(), 10)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Zero(t, count)
			require.Contains(t, err.Error(), "cancel expired payment orders")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "returns error if an expired order ID cannot be scanned", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT id\\s+FROM payment_order").WithArgs(10).
				WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("not-a-uuid"))
			repoExpectRollback(pool)

			count, err := repo.CancelExpiredOrders(context.Background(), 10)

			require.Error(t, err)
			require.Zero(t, count)
			require.Contains(t, err.Error(), "scan expired order ID")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "returns expired order selection iteration error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			rows := pgxmock.NewRows([]string{"id"}).AddRow(repoTestOrderID).CloseError(repoTestDBErr)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT id\\s+FROM payment_order").WithArgs(10).WillReturnRows(rows)
			repoExpectRollback(pool)

			count, err := repo.CancelExpiredOrders(context.Background(), 10)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Zero(t, count)
			require.Contains(t, err.Error(), "iterate expired orders")
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "rolls back whole batch if one selected order disappeared", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT id\\s+FROM payment_order").WithArgs(2).
				WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(repoTestOrderID))
			pool.ExpectQuery("FROM payment_order").WithArgs(repoTestOrderID).WillReturnError(pgx.ErrNoRows)
			repoExpectRollback(pool)

			count, err := repo.CancelExpiredOrders(context.Background(), 2)

			require.ErrorIs(t, err, models.ErrPaymentOrderNotFound)
			require.Zero(t, count)
			require.NoError(t, pool.ExpectationsWereMet())
		}},

		{name: "rolls back whole batch on active attempt cancellation error", run: func(t *testing.T) {
			repo, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			pool.ExpectQuery("SELECT id\\s+FROM payment_order").WithArgs(1).
				WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(repoTestOrderID))
			pool.ExpectQuery("FROM payment_order").WithArgs(repoTestOrderID).
				WillReturnRows(repoOrderRow(repoTestOrderID, models.StatusProcessing, nil))
			pool.ExpectQuery("FROM payment_attempt").WithArgs(repoTestOrderID).
				WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(repoTestAttemptID))
			pool.ExpectQuery("UPDATE payment_order").WithArgs(repoTestOrderID).
				WillReturnRows(pgxmock.NewRows([]string{"canceled_at", "updated_at"}).AddRow(&repoTestTime, repoTestTime))
			pool.ExpectExec("UPDATE payment_attempt").WithArgs(repoTestAttemptID, repoTestOrderID, reasonOrderExpired).
				WillReturnError(repoTestDBErr)
			repoExpectRollback(pool)

			count, err := repo.CancelExpiredOrders(context.Background(), 1)

			require.ErrorIs(t, err, repoTestDBErr)
			require.Zero(t, count)
			require.NoError(t, pool.ExpectationsWereMet())
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestPaymentRepositoryFull_ChangeAttemptStatus(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "rejects unsupported target status", run: func(t *testing.T) {
			_, pool := newPaymentRepositoryTest(t)
			repoExpectBegin(pool)
			repoExpectRollback(pool)

			err := pgx.BeginTxFunc(context.Background(), pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
				return changeAttemptStatus(context.Background(), tx, repoTestOrderID, repoTestAttemptID,
					models.AttemptStatusCreated, "")
			})

			require.ErrorIs(t, err, models.ErrPaymentAttemptValidation)
			require.NoError(t, pool.ExpectationsWereMet())
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestPaymentRepositoryFull_UtilityFunctions(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "uniqueUUIDs preserves first occurrence order", run: func(t *testing.T) {
			got := uniqueUUIDs([]uuid.UUID{
				repoTestOrderID,
				repoTestSecondOrderID,
				repoTestOrderID,
				repoTestAttemptID,
				repoTestSecondOrderID,
			})
			require.Equal(t, []uuid.UUID{repoTestOrderID, repoTestSecondOrderID, repoTestAttemptID}, got)
		}},

		{name: "uniqueUUIDs returns an empty non-nil slice", run: func(t *testing.T) {
			got := uniqueUUIDs(nil)
			require.NotNil(t, got)
			require.Empty(t, got)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

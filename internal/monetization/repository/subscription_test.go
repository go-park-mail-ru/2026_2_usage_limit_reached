package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var subscriptionTiersQuery = testutils.SQLInOrder(
	"FROM subscription_type st",
	"interval_unit = 'day'",
	"interval_count = 30",
	"currency = 'RUB'",
	"WHERE st.author_id = $1",
	"(bp.amount_minor, st.id) > ($2::bigint, $3::uuid)",
	"ORDER BY bp.amount_minor, st.id",
	"LIMIT $4",
)

func TestMonetizationRepository_GetSubscriptionTiers(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	tierID := uuid.New()
	createdAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	archivedAt := createdAt.Add(time.Hour)

	tests := []struct {
		name   string
		cursor *dto.SubscriptionTierCursor
		args   []any
	}{
		{
			name: "first page",
			args: []any{authorID, (*int64)(nil), (*uuid.UUID)(nil), 21},
		},
		{
			name:   "cursor page",
			cursor: &dto.SubscriptionTierCursor{PriceMinor: 1200, TierID: tierID},
			args:   []any{authorID, ptr(int64(1200)), &tierID, 21},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)
			pool.ExpectQuery(subscriptionTiersQuery).
				WithArgs(tt.args...).
				WillReturnRows(pgxmock.NewRows([]string{
					"id",
					"author_id",
					"name",
					"description",
					"amount_minor",
					"currency",
					"retired_at",
					"created_at",
					"updated_at",
				}).AddRow(
					tierID,
					authorID,
					"Bronze",
					"Bronze tier",
					int64(1200),
					"RUB",
					&archivedAt,
					createdAt,
					createdAt,
				))

			got, err := repo.GetSubscriptionTiers(context.Background(), authorID, tt.cursor, 21)

			require.NoError(t, err)
			require.Equal(t, []*models.SubscriptionTier{{
				ID:          tierID,
				AuthorID:    authorID,
				Name:        "Bronze",
				Description: "Bronze tier",
				PriceMinor:  1200,
				Currency:    "RUB",
				ArchivedAt:  &archivedAt,
				CreatedAt:   createdAt,
				UpdatedAt:   createdAt,
			}}, got)
		})
	}
}

func TestMonetizationRepository_GetSubscriptionTiers_QueryError(t *testing.T) {
	t.Parallel()

	repo, pool := newRepository(t)
	authorID := uuid.New()
	queryErr := errors.New("subscription tiers unavailable")

	pool.ExpectQuery(subscriptionTiersQuery).
		WithArgs(authorID, (*int64)(nil), (*uuid.UUID)(nil), 21).
		WillReturnError(queryErr)

	got, err := repo.GetSubscriptionTiers(context.Background(), authorID, nil, 21)

	require.Nil(t, got)
	require.ErrorIs(t, err, queryErr)
}

func ptr[T any](v T) *T {
	return &v
}

func TestMonetizationRepository_CreateSubscriptionTier_Atomic(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	authorID := repoTestAuthorID
	tierID := uuid.New()
	req := dto.CreateSubscriptionTierRequest{
		Name:        "Bronze",
		Description: "Bronze tier",
		PriceMinor:  50000,
	}
	repoExpectBegin(pool)
	pool.ExpectQuery("SELECT EXISTS").WithArgs(
		authorID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, subscriptionTierCurrency, req.PriceMinor,
	).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	pool.ExpectQuery("INSERT INTO subscription_type \\(author_id, name, description\\)").WithArgs(
		authorID, req.Name, req.Description,
	).WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
		AddRow(tierID, repoTestTime, repoTestTime))
	pool.ExpectExec("INSERT INTO billing_plan").WithArgs(
		tierID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, req.PriceMinor, subscriptionTierCurrency,
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	repoExpectCommit(pool)

	got, err := repo.CreateSubscriptionTier(context.Background(), authorID, req)

	require.NoError(t, err)
	require.Equal(t, &models.SubscriptionTier{
		ID:          tierID,
		AuthorID:    authorID,
		Name:        req.Name,
		Description: req.Description,
		PriceMinor:  req.PriceMinor,
		Currency:    subscriptionTierCurrency,
		CreatedAt:   repoTestTime,
		UpdatedAt:   repoTestTime,
	}, got)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_CreateSubscriptionTier_AuthorNotFound(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	req := dto.CreateSubscriptionTierRequest{Name: "Bronze", Description: "Bronze tier", PriceMinor: 50000}
	repoExpectBegin(pool)
	pool.ExpectQuery("SELECT EXISTS").WithArgs(
		repoTestAuthorID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, subscriptionTierCurrency, req.PriceMinor,
	).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	pool.ExpectQuery("INSERT INTO subscription_type \\(author_id, name, description\\)").WithArgs(
		repoTestAuthorID, req.Name, req.Description,
	).WillReturnError(&pgconn.PgError{Code: "23503"})
	repoExpectRollback(pool)

	got, err := repo.CreateSubscriptionTier(context.Background(), repoTestAuthorID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, models.ErrAuthorNotFound)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_CreateSubscriptionTier_DuplicateName(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	req := dto.CreateSubscriptionTierRequest{Name: "Bronze", Description: "Bronze tier", PriceMinor: 50000}
	repoExpectBegin(pool)
	pool.ExpectQuery("SELECT EXISTS").WithArgs(
		repoTestAuthorID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, subscriptionTierCurrency, req.PriceMinor,
	).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	pool.ExpectQuery("INSERT INTO subscription_type \\(author_id, name, description\\)").WithArgs(
		repoTestAuthorID, req.Name, req.Description,
	).WillReturnError(&pgconn.PgError{Code: "23505"})
	repoExpectRollback(pool)

	got, err := repo.CreateSubscriptionTier(context.Background(), repoTestAuthorID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, models.ErrSubscriptionTierConflict)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_CreateSubscriptionTier_DuplicateActivePrice(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	req := dto.CreateSubscriptionTierRequest{Name: "Bronze", Description: "Bronze tier", PriceMinor: 50000}
	repoExpectBegin(pool)
	pool.ExpectQuery("SELECT EXISTS").WithArgs(
		repoTestAuthorID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, subscriptionTierCurrency, req.PriceMinor,
	).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
	repoExpectRollback(pool)

	got, err := repo.CreateSubscriptionTier(context.Background(), repoTestAuthorID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, models.ErrSubscriptionTierConflict)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_CreateSubscriptionTier_RollsBackIfPlanInsertFails(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	tierID := uuid.New()
	req := dto.CreateSubscriptionTierRequest{Name: "Bronze", Description: "Bronze tier", PriceMinor: 50000}
	repoExpectBegin(pool)
	pool.ExpectQuery("SELECT EXISTS").WithArgs(
		repoTestAuthorID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, subscriptionTierCurrency, req.PriceMinor,
	).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	pool.ExpectQuery("INSERT INTO subscription_type \\(author_id, name, description\\)").WithArgs(
		repoTestAuthorID, req.Name, req.Description,
	).WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
		AddRow(tierID, repoTestTime, repoTestTime))
	planErr := errors.New("billing plan unavailable")
	pool.ExpectExec("INSERT INTO billing_plan").WithArgs(
		tierID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, req.PriceMinor, subscriptionTierCurrency,
	).WillReturnError(planErr)
	repoExpectRollback(pool)

	got, err := repo.CreateSubscriptionTier(context.Background(), repoTestAuthorID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, planErr)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_GetSubscriptionOptions_GuestGetsOpenTiers(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	authorID := uuid.New()
	tierID := uuid.New()
	pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	pool.ExpectQuery("FROM subscription_type st").
		WithArgs(authorID, (*int64)(nil), (*int64)(nil), (*int64)(nil), (*uuid.UUID)(nil), 21).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "author_id", "name", "description", "amount_minor", "currency", "retired_at", "created_at", "updated_at",
		}).AddRow(
			tierID, authorID, "Bronze", "Bronze tier", int64(50000), "RUB", nil, repoTestTime, repoTestTime,
		))
	repoExpectCommit(pool)

	tiers, current, err := repo.GetSubscriptionOptions(context.Background(), uuid.Nil, authorID, nil, nil, 21)

	require.NoError(t, err)
	require.Nil(t, current)
	require.Equal(t, []*models.SubscriptionTier{{
		ID:          tierID,
		AuthorID:    authorID,
		Name:        "Bronze",
		Description: "Bronze tier",
		PriceMinor:  50000,
		Currency:    "RUB",
		CreatedAt:   repoTestTime,
		UpdatedAt:   repoTestTime,
	}}, tiers)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_GetSubscriptionOptions_PostAlreadyAccessible(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	authorID := uuid.New()
	readerID := uuid.New()
	postID := uuid.New()
	requiredTierID := uuid.New()
	subscriptionID := uuid.New()
	periodEndsAt := time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC)

	pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	pool.ExpectQuery("FROM post_access_stage").WithArgs(postID).
		WillReturnRows(pgxmock.NewRows([]string{"subscription_type_id"}).AddRow(&requiredTierID))
	pool.ExpectQuery("FROM billing_plan").WithArgs(
		requiredTierID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, subscriptionTierCurrency,
	).WillReturnRows(pgxmock.NewRows([]string{"amount_minor"}).AddRow(int64(50000)))
	pool.ExpectQuery("FROM subscription_period_with_end").WithArgs(readerID, authorID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "id", "name", "tier_status", "amount_minor", "currency", "period_end",
		}).AddRow(
			subscriptionID, requiredTierID, "Bronze", "open", int64(50000), "RUB", periodEndsAt,
		))
	repoExpectCommit(pool)

	tiers, current, err := repo.GetSubscriptionOptions(context.Background(), readerID, authorID, &postID, nil, 21)

	require.NoError(t, err)
	require.Empty(t, tiers)
	require.Equal(t, &models.CurrentSubscription{
		ID:           subscriptionID,
		TierID:       requiredTierID,
		TierName:     "Bronze",
		TierStatus:   "open",
		PriceMinor:   50000,
		Currency:     "RUB",
		PeriodEndsAt: periodEndsAt,
	}, current)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_GetSubscriptionOptions_FiltersByPostTierPrice(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	authorID := uuid.New()
	postID := uuid.New()
	requiredTierID := uuid.New()
	optionTierID := uuid.New()
	pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	pool.ExpectQuery("FROM post_access_stage").WithArgs(postID).
		WillReturnRows(pgxmock.NewRows([]string{"subscription_type_id"}).AddRow(&requiredTierID))
	pool.ExpectQuery("FROM billing_plan").WithArgs(
		requiredTierID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, subscriptionTierCurrency,
	).WillReturnRows(pgxmock.NewRows([]string{"amount_minor"}).AddRow(int64(50000)))
	pool.ExpectQuery("FROM subscription_type st").
		WithArgs(authorID, ptr(int64(50000)), (*int64)(nil), (*int64)(nil), (*uuid.UUID)(nil), 21).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "author_id", "name", "description", "amount_minor", "currency", "retired_at", "created_at", "updated_at",
		}).AddRow(
			optionTierID, authorID, "Silver", "Silver tier", int64(70000), "RUB", nil, repoTestTime, repoTestTime,
		))
	repoExpectCommit(pool)

	tiers, current, err := repo.GetSubscriptionOptions(context.Background(), uuid.Nil, authorID, &postID, nil, 21)

	require.NoError(t, err)
	require.Nil(t, current)
	require.Len(t, tiers, 1)
	require.Equal(t, optionTierID, tiers[0].ID)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_GetSubscriptionOptions_PublicPostStageReturnsNoOptions(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	authorID := uuid.New()
	postID := uuid.New()
	pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	pool.ExpectQuery("FROM post_access_stage").WithArgs(postID).
		WillReturnRows(pgxmock.NewRows([]string{"subscription_type_id"}).AddRow(nil))
	repoExpectCommit(pool)

	tiers, current, err := repo.GetSubscriptionOptions(context.Background(), uuid.Nil, authorID, &postID, nil, 21)

	require.NoError(t, err)
	require.Empty(t, tiers)
	require.Nil(t, current)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_GetSubscriptionOptions_MissingPostAccessScheduleDoesNotBlockOptions(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	authorID := uuid.New()
	postID := uuid.New()
	pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	pool.ExpectQuery("FROM post_access_stage").WithArgs(postID).
		WillReturnRows(pgxmock.NewRows([]string{"subscription_type_id"}))
	pool.ExpectQuery("FROM post_access").WithArgs(postID).
		WillReturnRows(pgxmock.NewRows([]string{"subscription_type_id"}))
	tierID := uuid.New()
	pool.ExpectQuery("FROM subscription_type st").
		WithArgs(authorID, (*int64)(nil), (*int64)(nil), (*int64)(nil), (*uuid.UUID)(nil), 21).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "author_id", "name", "description", "amount_minor", "currency", "retired_at", "created_at", "updated_at",
		}).AddRow(
			tierID, authorID, "Bronze", "Bronze tier", int64(50000), "RUB", nil, repoTestTime, repoTestTime,
		))
	repoExpectCommit(pool)

	tiers, current, err := repo.GetSubscriptionOptions(context.Background(), uuid.Nil, authorID, &postID, nil, 21)

	require.NoError(t, err)
	require.Len(t, tiers, 1)
	require.Nil(t, current)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_GetSubscriptionOptions_QueryError(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	authorID := uuid.New()
	queryErr := errors.New("subscription options unavailable")
	pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	pool.ExpectQuery("FROM subscription_type st").
		WithArgs(authorID, (*int64)(nil), (*int64)(nil), (*int64)(nil), (*uuid.UUID)(nil), 21).
		WillReturnError(queryErr)
	repoExpectRollback(pool)

	tiers, current, err := repo.GetSubscriptionOptions(context.Background(), uuid.Nil, authorID, nil, nil, 21)

	require.Nil(t, tiers)
	require.Nil(t, current)
	require.ErrorIs(t, err, queryErr)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_GetSubscriptionOptions_Cursor(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	authorID := uuid.New()
	tierID := uuid.New()
	cursor := &dto.SubscriptionTierCursor{PriceMinor: 50000, TierID: tierID}
	pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	pool.ExpectQuery("FROM subscription_type st").
		WithArgs(authorID, (*int64)(nil), (*int64)(nil), &cursor.PriceMinor, &cursor.TierID, 21).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "author_id", "name", "description", "amount_minor", "currency", "retired_at", "created_at", "updated_at",
		}).AddRow(
			tierID, authorID, "Silver", "Silver tier", int64(70000), "RUB", nil, repoTestTime, repoTestTime,
		))
	repoExpectCommit(pool)

	tiers, current, err := repo.GetSubscriptionOptions(context.Background(), uuid.Nil, authorID, nil, cursor, 21)

	require.NoError(t, err)
	require.Nil(t, current)
	require.Len(t, tiers, 1)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_UpdateSubscriptionTier_ChangesPriceWithoutReordering(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	tierID := uuid.New()
	planID := uuid.New()
	req := dto.UpdateSubscriptionTierRequest{
		Name:        "Bronze",
		Description: "Bronze tier",
		PriceMinor:  55000,
		Status:      "open",
	}
	repoExpectBegin(pool)
	pool.ExpectQuery("FROM subscription_type").WithArgs(tierID, repoTestAuthorID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "author_id", "name", "description", "retired_at", "created_at", "updated_at",
		}).AddRow(tierID, repoTestAuthorID, "Bronze", "Bronze tier", nil, repoTestTime, repoTestTime))
	pool.ExpectQuery("FROM billing_plan").WithArgs(
		tierID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, subscriptionTierCurrency,
	).WillReturnRows(pgxmock.NewRows([]string{"id", "amount_minor", "currency"}).
		AddRow(planID, int64(50000), subscriptionTierCurrency))
	pool.ExpectQuery("SELECT EXISTS").WithArgs(
		repoTestAuthorID,
		tierID,
		billingPlanIntervalUnitDay,
		billingPlanIntervalCount30,
		subscriptionTierCurrency,
		req.PriceMinor,
		int64(50000),
	).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	pool.ExpectExec("UPDATE billing_plan").WithArgs(req.PriceMinor, planID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	pool.ExpectQuery("UPDATE subscription_type").WithArgs(
		tierID, repoTestAuthorID, req.Name, req.Description, req.Status,
	).WillReturnRows(pgxmock.NewRows([]string{"retired_at", "updated_at"}).
		AddRow(nil, repoTestTime))
	repoExpectCommit(pool)

	got, err := repo.UpdateSubscriptionTier(context.Background(), repoTestAuthorID, tierID, req)

	require.NoError(t, err)
	require.Equal(t, &models.SubscriptionTier{
		ID:          tierID,
		AuthorID:    repoTestAuthorID,
		Name:        req.Name,
		Description: req.Description,
		PriceMinor:  req.PriceMinor,
		Currency:    subscriptionTierCurrency,
		CreatedAt:   repoTestTime,
		UpdatedAt:   repoTestTime,
	}, got)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_UpdateSubscriptionTier_RejectsArchivedTier(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	tierID := uuid.New()
	req := dto.UpdateSubscriptionTierRequest{
		Name:        "Bronze",
		Description: "Bronze tier",
		PriceMinor:  50000,
		Status:      "open",
	}
	repoExpectBegin(pool)
	archivedAt := time.Now()
	pool.ExpectQuery("FROM subscription_type").WithArgs(tierID, repoTestAuthorID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "author_id", "name", "description", "retired_at", "created_at", "updated_at",
		}).AddRow(tierID, repoTestAuthorID, "Bronze", "Bronze tier", &archivedAt, repoTestTime, repoTestTime))
	repoExpectRollback(pool)

	got, err := repo.UpdateSubscriptionTier(context.Background(), repoTestAuthorID, tierID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, models.ErrSubscriptionTierConflict)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_UpdateSubscriptionTier_RejectsReorderingPrice(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	tierID := uuid.New()
	planID := uuid.New()
	req := dto.UpdateSubscriptionTierRequest{
		Name:        "Bronze",
		Description: "Bronze tier",
		PriceMinor:  70000,
		Status:      "open",
	}
	repoExpectBegin(pool)
	pool.ExpectQuery("FROM subscription_type").WithArgs(tierID, repoTestAuthorID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "author_id", "name", "description", "retired_at", "created_at", "updated_at",
		}).AddRow(tierID, repoTestAuthorID, "Bronze", "Bronze tier", nil, repoTestTime, repoTestTime))
	pool.ExpectQuery("FROM billing_plan").WithArgs(
		tierID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, subscriptionTierCurrency,
	).WillReturnRows(pgxmock.NewRows([]string{"id", "amount_minor", "currency"}).
		AddRow(planID, int64(50000), subscriptionTierCurrency))
	pool.ExpectQuery("SELECT EXISTS").WithArgs(
		repoTestAuthorID,
		tierID,
		billingPlanIntervalUnitDay,
		billingPlanIntervalCount30,
		subscriptionTierCurrency,
		req.PriceMinor,
		int64(50000),
	).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
	repoExpectRollback(pool)

	got, err := repo.UpdateSubscriptionTier(context.Background(), repoTestAuthorID, tierID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, models.ErrSubscriptionTierConflict)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_UpdateSubscriptionTier_AllowsArchiveAtMatchingPrice(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	tierID := uuid.New()
	planID := uuid.New()
	req := dto.UpdateSubscriptionTierRequest{
		Name:        "Bronze",
		Description: "Bronze tier",
		PriceMinor:  70000,
		Status:      "archived",
	}
	repoExpectBegin(pool)
	pool.ExpectQuery("FROM subscription_type").WithArgs(tierID, repoTestAuthorID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "author_id", "name", "description", "retired_at", "created_at", "updated_at",
		}).AddRow(tierID, repoTestAuthorID, "Bronze", "Bronze tier", nil, repoTestTime, repoTestTime))
	pool.ExpectQuery("FROM billing_plan").WithArgs(
		tierID, billingPlanIntervalUnitDay, billingPlanIntervalCount30, subscriptionTierCurrency,
	).WillReturnRows(pgxmock.NewRows([]string{"id", "amount_minor", "currency"}).
		AddRow(planID, int64(50000), subscriptionTierCurrency))
	pool.ExpectExec("UPDATE billing_plan").WithArgs(req.PriceMinor, planID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	archivedAt := repoTestTime
	pool.ExpectQuery("UPDATE subscription_type").WithArgs(
		tierID, repoTestAuthorID, req.Name, req.Description, req.Status,
	).WillReturnRows(pgxmock.NewRows([]string{"retired_at", "updated_at"}).
		AddRow(&archivedAt, repoTestTime))
	repoExpectCommit(pool)

	got, err := repo.UpdateSubscriptionTier(context.Background(), repoTestAuthorID, tierID, req)

	require.NoError(t, err)
	require.Equal(t, &models.SubscriptionTier{
		ID:          tierID,
		AuthorID:    repoTestAuthorID,
		Name:        req.Name,
		Description: req.Description,
		PriceMinor:  req.PriceMinor,
		Currency:    subscriptionTierCurrency,
		ArchivedAt:  &archivedAt,
		CreatedAt:   repoTestTime,
		UpdatedAt:   repoTestTime,
	}, got)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_CreateSubscriptionWithPayment(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	readerID := repoTestPayerID
	authorID := repoTestAuthorID
	tierID := uuid.New()
	planID := uuid.New()
	subscriptionID := uuid.New()
	startedAt := repoTestTime
	periodEndsAt := startedAt.AddDate(0, 0, billingPlanIntervalCount30)
	expiresAt := repoTestTime.AddDate(0, 0, 1)
	req := dto.CreateSubscriptionWithPayment{
		ReaderID:           readerID,
		AuthorID:           authorID,
		Action:             "subscribe",
		TierID:             tierID,
		ExpectedPriceMinor: 50000,
		RequestKey:         "subscription-purchase-key",
		ExpiresAt:          &expiresAt,
	}

	repoExpectBegin(pool)
	pool.ExpectQuery("INSERT INTO payment_order").WithArgs(
		readerID,
		req.RequestKey,
		models.PurposeSubscriptionPeriod,
		models.OriginManual,
		req.ExpectedPriceMinor,
		subscriptionTierCurrency,
		req.ExpiresAt,
	).WillReturnRows(pgxmock.NewRows(repoTestOrderColumns).AddRow(
		repoTestOrderID,
		readerID,
		req.RequestKey,
		models.PurposeSubscriptionPeriod,
		models.OriginManual,
		req.ExpectedPriceMinor,
		subscriptionTierCurrency,
		models.StatusPending,
		nil,
		req.ExpiresAt,
		nil,
		nil,
		repoTestTime,
		repoTestTime,
	))
	pool.ExpectQuery("FROM subscription_type").WithArgs(tierID, authorID).
		WillReturnRows(pgxmock.NewRows([]string{"retired_at"}).AddRow(nil))
	pool.ExpectQuery("FROM billing_plan").WithArgs(
		tierID,
		billingPlanIntervalUnitDay,
		billingPlanIntervalCount30,
		subscriptionTierCurrency,
	).WillReturnRows(pgxmock.NewRows([]string{"id", "amount_minor"}).AddRow(planID, req.ExpectedPriceMinor))
	pool.ExpectQuery("FROM subscription s").
		WithArgs(readerID, authorID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "id", "amount_minor",
		}))
	pool.ExpectQuery("FROM subscription s").
		WithArgs(readerID, authorID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "status"}))
	pool.ExpectQuery("INSERT INTO subscription").WithArgs(readerID, planID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(subscriptionID))
	pool.ExpectQuery("SELECT EXISTS").WithArgs(subscriptionID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	pool.ExpectExec("INSERT INTO subscription_period").WithArgs(
		repoTestOrderID,
		subscriptionID,
		planID,
		false,
		req.Action,
		(*uuid.UUID)(nil),
		(*uuid.UUID)(nil),
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	pool.ExpectExec("INSERT INTO payment_attempt").WithArgs(repoTestOrderID, defaultPaymentProvider).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	pool.ExpectQuery("FROM subscription_period_with_end spe").WithArgs(repoTestOrderID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tier_id", "name", "tier_status", "amount_minor", "currency",
			"period_start", "period_end", "author_id", "user_id",
			"request_action", "expected_current_tier_id", "has_post_id", "post_id",
		}).AddRow(
			subscriptionID,
			tierID,
			"Bronze",
			"open",
			req.ExpectedPriceMinor,
			subscriptionTierCurrency,
			startedAt,
			periodEndsAt,
			authorID,
			readerID,
			req.Action,
			uuid.Nil,
			false,
			uuid.Nil,
		))
	repoExpectCommit(pool)

	subscription, order, err := repo.CreateSubscriptionWithPayment(context.Background(), req)

	require.NoError(t, err)
	require.Equal(t, &models.CurrentSubscription{
		ID:           subscriptionID,
		TierID:       tierID,
		TierName:     "Bronze",
		TierStatus:   "open",
		PriceMinor:   req.ExpectedPriceMinor,
		Currency:     subscriptionTierCurrency,
		PeriodEndsAt: periodEndsAt,
	}, subscription)
	require.Equal(t, repoTestOrderID, order.ID)
	require.Equal(t, models.PurposeSubscriptionPeriod, order.Purpose)
	require.Equal(t, models.StatusPending, order.Status)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_CreateSubscriptionWithPayment_IdempotentRetry(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	subscriptionID := uuid.New()
	tierID := uuid.New()
	expectedCurrentTierID := uuid.New()
	postID := uuid.New()
	expiresAt := repoTestTime.AddDate(0, 0, 1)
	req := dto.CreateSubscriptionWithPayment{
		ReaderID:               repoTestPayerID,
		AuthorID:               repoTestAuthorID,
		Action:                 "upgrade",
		TierID:                 tierID,
		ExpectedSubscriptionID: &subscriptionID,
		ExpectedCurrentTierID:  &expectedCurrentTierID,
		ExpectedPriceMinor:     50000,
		PostID:                 &postID,
		RequestKey:             "subscription-upgrade-key",
		ExpiresAt:              &expiresAt,
	}
	periodEndsAt := repoTestTime.AddDate(0, 0, billingPlanIntervalCount30)

	repoExpectBegin(pool)
	pool.ExpectQuery("INSERT INTO payment_order").WithArgs(
		req.ReaderID,
		req.RequestKey,
		models.PurposeSubscriptionPeriod,
		models.OriginManual,
		req.ExpectedPriceMinor,
		subscriptionTierCurrency,
		req.ExpiresAt,
	).WillReturnError(pgx.ErrNoRows)
	pool.ExpectQuery("FROM payment_order").WithArgs(req.ReaderID, req.RequestKey).
		WillReturnRows(pgxmock.NewRows(repoTestOrderColumns).AddRow(
			repoTestOrderID,
			req.ReaderID,
			req.RequestKey,
			models.PurposeSubscriptionPeriod,
			models.OriginManual,
			req.ExpectedPriceMinor,
			subscriptionTierCurrency,
			models.StatusPaid,
			ptr(repoTestAttemptID),
			req.ExpiresAt,
			ptr(repoTestTime),
			nil,
			repoTestTime,
			repoTestTime,
		))
	pool.ExpectQuery("FROM subscription_period_with_end spe").WithArgs(repoTestOrderID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tier_id", "name", "tier_status", "amount_minor", "currency",
			"period_start", "period_end", "author_id", "user_id",
			"request_action", "expected_current_tier_id", "has_post_id", "post_id",
		}).AddRow(
			subscriptionID,
			tierID,
			"Gold",
			"open",
			req.ExpectedPriceMinor,
			subscriptionTierCurrency,
			repoTestTime,
			periodEndsAt,
			req.AuthorID,
			req.ReaderID,
			req.Action,
			expectedCurrentTierID,
			true,
			postID,
		))
	repoExpectCommit(pool)

	got, order, err := repo.CreateSubscriptionWithPayment(context.Background(), req)

	require.NoError(t, err)
	require.Equal(t, subscriptionID, got.ID)
	require.Equal(t, tierID, got.TierID)
	require.Equal(t, repoTestOrderID, order.ID)
	require.Equal(t, models.StatusPaid, order.Status)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_ActivateSubscription(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	subscriptionID := uuid.New()
	tierID := uuid.New()
	planID := uuid.New()
	periodEndsAt := repoTestTime.AddDate(0, 0, billingPlanIntervalCount30)
	current := &models.CurrentSubscription{
		ID:           subscriptionID,
		TierID:       tierID,
		TierName:     "Bronze",
		TierStatus:   "open",
		PriceMinor:   50000,
		Currency:     subscriptionTierCurrency,
		PeriodEndsAt: periodEndsAt,
	}

	repoExpectBegin(pool)
	pool.ExpectQuery("FROM payment_order po").WithArgs(repoTestOrderID).
		WillReturnRows(pgxmock.NewRows([]string{
			"order_status", "subscription_id", "billing_plan_id", "period_start", "subscription_status",
			"tier_id", "tier_name", "tier_status", "expected_amount_minor", "currency", "period_end",
		}).AddRow(
			models.StatusPaid,
			subscriptionID,
			planID,
			repoTestTime,
			"pending",
			tierID,
			current.TierName,
			current.TierStatus,
			current.PriceMinor,
			current.Currency,
			periodEndsAt,
		))
	pool.ExpectQuery("SELECT EXISTS").WithArgs(subscriptionID, repoTestTime).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	pool.ExpectExec("UPDATE subscription_period").WithArgs(subscriptionID, repoTestOrderID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	pool.ExpectExec("UPDATE subscription_period").WithArgs(repoTestOrderID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	pool.ExpectExec("UPDATE subscription").WithArgs(subscriptionID, planID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	repoExpectCommit(pool)

	got, err := repo.ActivateSubscription(context.Background(), repoTestOrderID)

	require.NoError(t, err)
	require.Equal(t, current, got)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestMonetizationRepository_ActivateSubscription_DoesNotReactivateSupersededPeriod(t *testing.T) {
	t.Parallel()

	repo, pool := newPaymentRepositoryTest(t)
	subscriptionID := uuid.New()
	tierID := uuid.New()
	planID := uuid.New()
	periodEndsAt := repoTestTime.AddDate(0, 0, billingPlanIntervalCount30)
	current := &models.CurrentSubscription{
		ID:           subscriptionID,
		TierID:       tierID,
		TierName:     "Bronze",
		TierStatus:   "open",
		PriceMinor:   50000,
		Currency:     subscriptionTierCurrency,
		PeriodEndsAt: periodEndsAt,
	}

	repoExpectBegin(pool)
	pool.ExpectQuery("FROM payment_order po").WithArgs(repoTestOrderID).
		WillReturnRows(pgxmock.NewRows([]string{
			"order_status", "subscription_id", "billing_plan_id", "period_start", "subscription_status",
			"tier_id", "tier_name", "tier_status", "expected_amount_minor", "currency", "period_end",
		}).AddRow(
			models.StatusPaid,
			subscriptionID,
			planID,
			repoTestTime,
			"active",
			tierID,
			current.TierName,
			current.TierStatus,
			current.PriceMinor,
			current.Currency,
			periodEndsAt,
		))
	pool.ExpectQuery("SELECT EXISTS").WithArgs(subscriptionID, repoTestTime).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
	repoExpectCommit(pool)

	got, err := repo.ActivateSubscription(context.Background(), repoTestOrderID)

	require.NoError(t, err)
	require.Equal(t, current, got)
	require.NoError(t, pool.ExpectationsWereMet())
}

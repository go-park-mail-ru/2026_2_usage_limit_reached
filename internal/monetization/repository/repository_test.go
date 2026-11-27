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
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var (
	donationPayerID  = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	donationAuthorID = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	donationOrderID  = uuid.MustParse("00000000-0000-4000-8000-000000000301")

	donationTime      = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	donationUpdatedAt = donationTime.Add(time.Minute)
	donationMessage   = "Спасибо за посты!"

	technicalErr = errors.New("donation storage unavailable")

	markDonationAsPaidQuery = testutils.SQLInOrder(
		"UPDATE donation",
		"SET paid = TRUE",
		"WHERE payment_order_id = $1",
		"AND paid = FALSE",
	)

	receivedQuery = testutils.SQLInOrder(
		"FROM donation",
		"WHERE author_id = $1",
		"AND paid = TRUE",
		"(created_at, payment_order_id) < ($2::timestamptz, $3::uuid)",
		"ORDER BY created_at DESC, payment_order_id DESC",
		"LIMIT $4",
	)

	sentQuery = testutils.SQLInOrder(
		"FROM donation",
		"WHERE donor_id = $1",
		"(created_at, payment_order_id) < ($2::timestamptz, $3::uuid)",
		"ORDER BY created_at DESC, payment_order_id DESC",
		"LIMIT $4",
	)

	donationColumns = []string{
		"payment_order_id",
		"donor_id",
		"author_id",
		"message",
		"is_anonymous",
		"created_at",
		"updated_at",
	}
)

func donationRow(paymentOrderID, donorID, authorID uuid.UUID, message *string, isAnonymous bool) []any {
	return []any{
		paymentOrderID,
		donorID,
		authorID,
		message,
		isAnonymous,
		donationTime,
		donationUpdatedAt,
	}
}

func TestMonetizationRepository_MarkPaid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		rowsAffected int64
	}{
		{
			name:         "updated",
			rowsAffected: 1,
		},
		{
			name:         "already paid",
			rowsAffected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)

			pool.ExpectExec(markDonationAsPaidQuery).
				WithArgs(donationOrderID).
				WillReturnResult(pgxmock.NewResult("UPDATE", tt.rowsAffected))

			err := repo.MarkPaid(context.Background(), donationOrderID)

			require.NoError(t, err)
		})
	}
}

func TestMonetizationRepository_MarkPaid_Error(t *testing.T) {
	t.Parallel()

	repo, pool := newRepository(t)

	pool.ExpectExec(markDonationAsPaidQuery).
		WithArgs(donationOrderID).
		WillReturnError(technicalErr)

	err := repo.MarkPaid(context.Background(), donationOrderID)

	require.ErrorIs(t, err, technicalErr)
}

type listMethod struct {
	name   string
	query  string
	userID uuid.UUID
	call   func(repo *MonetizationRepository, userID uuid.UUID, cursor *dto.DonationCursor) ([]*models.Donation, error)
}

func listMethods() []listMethod {
	ctx := context.Background()

	return []listMethod{
		{
			name:   "GetReceivedDonations",
			query:  receivedQuery,
			userID: donationAuthorID,
			call: func(
				repo *MonetizationRepository,
				userID uuid.UUID,
				cursor *dto.DonationCursor,
			) ([]*models.Donation, error) {
				return repo.GetReceivedDonations(ctx, userID, cursor, 21)
			},
		},
		{
			name:   "GetSentDonations",
			query:  sentQuery,
			userID: donationPayerID,
			call: func(
				repo *MonetizationRepository,
				userID uuid.UUID,
				cursor *dto.DonationCursor,
			) ([]*models.Donation, error) {
				return repo.GetSentDonations(ctx, userID, cursor, 21)
			},
		},
	}
}

func TestMonetizationRepository_ListDonations_PaidFilter(t *testing.T) {
	t.Parallel()

	require.Contains(t, receivedQuery, "paid = TRUE")
	require.NotContains(t, sentQuery, "paid = TRUE")
}

func TestMonetizationRepository_ListDonations_CursorArgs(t *testing.T) {
	t.Parallel()

	cursor := &dto.DonationCursor{
		CreatedAt:  donationTime,
		DonationID: donationOrderID,
	}

	tests := []struct {
		name   string
		cursor *dto.DonationCursor
		args   func(userID uuid.UUID) []any
	}{
		{
			name: "first page",
			args: func(userID uuid.UUID) []any {
				return []any{
					userID,
					(*time.Time)(nil),
					(*uuid.UUID)(nil),
					21,
				}
			},
		},
		{
			name:   "next page",
			cursor: cursor,
			args: func(userID uuid.UUID) []any {
				return []any{
					userID,
					&cursor.CreatedAt,
					&cursor.DonationID,
					21,
				}
			},
		},
	}

	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				repo, pool := newRepository(t)

				pool.ExpectQuery(m.query).
					WithArgs(tt.args(m.userID)...).
					WillReturnRows(pgxmock.NewRows(donationColumns))

				got, err := m.call(repo, m.userID, tt.cursor)

				require.NoError(t, err)
				require.NotNil(t, got)
				require.Empty(t, got)
			})
		}
	}
}

func TestMonetizationRepository_ListDonations_Scan(t *testing.T) {
	t.Parallel()

	for _, m := range listMethods() {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)

			pool.ExpectQuery(m.query).
				WithArgs(
					m.userID,
					(*time.Time)(nil),
					(*uuid.UUID)(nil),
					21,
				).
				WillReturnRows(
					pgxmock.NewRows(donationColumns).
						AddRow(donationRow(
							donationOrderID,
							donationPayerID,
							donationAuthorID,
							&donationMessage,
							false,
						)...),
				)

			got, err := m.call(repo, m.userID, nil)

			require.NoError(t, err)
			require.Equal(t, []*models.Donation{
				{
					PaymentOrderID: donationOrderID,
					DonorID:        donationPayerID,
					AuthorID:       donationAuthorID,
					Message:        &donationMessage,
					IsAnonymous:    false,
					CreatedAt:      donationTime,
					UpdatedAt:      donationUpdatedAt,
				},
			}, got)
		})
	}
}

func TestMonetizationRepository_ListDonations_Errors(t *testing.T) {
	t.Parallel()

	for _, m := range listMethods() {
		t.Run(m.name+"/query failure", func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)

			pool.ExpectQuery(m.query).
				WithArgs(
					m.userID,
					(*time.Time)(nil),
					(*uuid.UUID)(nil),
					21,
				).
				WillReturnError(technicalErr)

			got, err := m.call(repo, m.userID, nil)

			require.Nil(t, got)
			require.ErrorIs(t, err, technicalErr)
		})

		t.Run(m.name+"/row iteration failure", func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)

			pool.ExpectQuery(m.query).
				WithArgs(
					m.userID,
					(*time.Time)(nil),
					(*uuid.UUID)(nil),
					21,
				).
				WillReturnRows(
					pgxmock.NewRows(donationColumns).
						AddRow(donationRow(
							donationOrderID,
							donationPayerID,
							donationAuthorID,
							&donationMessage,
							false,
						)...).
						RowError(0, technicalErr),
				)

			got, err := m.call(repo, m.userID, nil)

			require.Nil(t, got)
			require.ErrorIs(t, err, technicalErr)
		})

		t.Run(m.name+"/scan failure", func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)

			pool.ExpectQuery(m.query).
				WithArgs(
					m.userID,
					(*time.Time)(nil),
					(*uuid.UUID)(nil),
					21,
				).
				WillReturnRows(
					pgxmock.NewRows([]string{"payment_order_id"}).
						AddRow(donationOrderID),
				)

			got, err := m.call(repo, m.userID, nil)

			require.Nil(t, got)
			require.Error(t, err)
		})
	}
}

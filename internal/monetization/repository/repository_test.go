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

	txOptions = pgx.TxOptions{IsoLevel: pgx.ReadCommitted}

	insertDonationQuery = testutils.SQLInOrder(
		"INSERT INTO donation",
		"payment_order_id",
		"donor_id",
		"author_id",
		"message",
		"is_anonymous",
		"ON CONFLICT (payment_order_id) DO NOTHING",
		"RETURNING created_at, updated_at",
	)

	existingDonationQuery = testutils.SQLInOrder(
		"SELECT",
		"payment_order_id",
		"donor_id",
		"author_id",
		"message",
		"is_anonymous",
		"created_at",
		"updated_at",
		"FROM donation",
		"WHERE payment_order_id = $1",
	)

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

func donationRequest() *dto.CreateDonationRequest {
	return &dto.CreateDonationRequest{
		AuthorID:    donationAuthorID,
		Message:     &donationMessage,
		IsAnonymous: false,
	}
}

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

func TestDonationRepository_CreateDonation_NewDonation(t *testing.T) {
	t.Parallel()

	req := donationRequest()
	repo, pool := newRepository(t)

	donationCreated(pool, donationOrderID, donationPayerID, req)
	testutils.ExpectTxCommitted(pool)

	got, err := repo.CreateDonation(
		context.Background(),
		donationOrderID,
		donationPayerID,
		req,
	)

	require.NoError(t, err)
	require.Equal(t, &models.Donation{
		PaymentOrderID: donationOrderID,
		DonorID:        donationPayerID,
		AuthorID:       donationAuthorID,
		Message:        &donationMessage,
		IsAnonymous:    false,
		CreatedAt:      donationTime,
		UpdatedAt:      donationUpdatedAt,
	}, got)
}

func TestDonationRepository_CreateDonation_RepeatedRequest(t *testing.T) {
	t.Parallel()

	req := donationRequest()
	repo, pool := newRepository(t)

	pool.ExpectBeginTx(txOptions)

	expectInsertDonation(pool, donationOrderID, donationPayerID, req).
		WillReturnRows(pgxmock.NewRows([]string{"created_at", "updated_at"}))

	pool.ExpectQuery(existingDonationQuery).
		WithArgs(donationOrderID).
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

	testutils.ExpectTxCommitted(pool)

	got, err := repo.CreateDonation(
		context.Background(),
		donationOrderID,
		donationPayerID,
		req,
	)

	require.NoError(t, err)
	require.Equal(t, &models.Donation{
		PaymentOrderID: donationOrderID,
		DonorID:        donationPayerID,
		AuthorID:       donationAuthorID,
		Message:        &donationMessage,
		IsAnonymous:    false,
		CreatedAt:      donationTime,
		UpdatedAt:      donationUpdatedAt,
	}, got)
}

func TestDonationRepository_CreateDonation_IdempotencyConflict(t *testing.T) {
	t.Parallel()

	otherMessage := "Другое сообщение"

	tests := []struct {
		name        string
		donorID     uuid.UUID
		authorID    uuid.UUID
		message     *string
		isAnonymous bool
	}{
		{
			name:        "other donor",
			donorID:     uuid.New(),
			authorID:    donationAuthorID,
			message:     &donationMessage,
			isAnonymous: false,
		},
		{
			name:        "other author",
			donorID:     donationPayerID,
			authorID:    uuid.New(),
			message:     &donationMessage,
			isAnonymous: false,
		},
		{
			name:        "other message",
			donorID:     donationPayerID,
			authorID:    donationAuthorID,
			message:     &otherMessage,
			isAnonymous: false,
		},
		{
			name:        "message removed",
			donorID:     donationPayerID,
			authorID:    donationAuthorID,
			message:     nil,
			isAnonymous: false,
		},
		{
			name:        "other anonymity",
			donorID:     donationPayerID,
			authorID:    donationAuthorID,
			message:     &donationMessage,
			isAnonymous: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := donationRequest()
			repo, pool := newRepository(t)

			pool.ExpectBeginTx(txOptions)

			expectInsertDonation(pool, donationOrderID, donationPayerID, req).
				WillReturnRows(pgxmock.NewRows([]string{"created_at", "updated_at"}))

			pool.ExpectQuery(existingDonationQuery).
				WithArgs(donationOrderID).
				WillReturnRows(
					pgxmock.NewRows(donationColumns).
						AddRow(donationRow(
							donationOrderID,
							tt.donorID,
							tt.authorID,
							tt.message,
							tt.isAnonymous,
						)...),
				)

			testutils.ExpectTxRolledBack(pool)

			got, err := repo.CreateDonation(
				context.Background(),
				donationOrderID,
				donationPayerID,
				req,
			)

			require.Nil(t, got)
			require.ErrorIs(t, err, models.ErrIdempotencyKeyReused)
		})
	}
}

func TestDonationRepository_CreateDonation_ExistingDonationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rows    *pgxmock.Rows
		dbErr   error
		wantErr error
	}{
		{
			name:    "existing donation not found",
			rows:    pgxmock.NewRows(donationColumns),
			wantErr: models.ErrDonationNotFound,
		},
		{
			name:    "storage failure",
			dbErr:   technicalErr,
			wantErr: technicalErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := donationRequest()
			repo, pool := newRepository(t)

			pool.ExpectBeginTx(txOptions)

			expectInsertDonation(pool, donationOrderID, donationPayerID, req).
				WillReturnRows(pgxmock.NewRows([]string{"created_at", "updated_at"}))

			existing := pool.ExpectQuery(existingDonationQuery).
				WithArgs(donationOrderID)

			if tt.rows != nil {
				existing.WillReturnRows(tt.rows)
			} else {
				existing.WillReturnError(tt.dbErr)
			}

			testutils.ExpectTxRolledBack(pool)

			got, err := repo.CreateDonation(
				context.Background(),
				donationOrderID,
				donationPayerID,
				req,
			)

			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestDonationRepository_CreateDonation_InsertErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dbErr   error
		wantErr error
	}{
		{
			name:    "foreign key violation",
			dbErr:   testutils.PgError("23503"),
			wantErr: models.ErrDonationValidation,
		},
		{
			name:    "check violation",
			dbErr:   testutils.PgError("23514"),
			wantErr: models.ErrDonationValidation,
		},
		{
			name:    "unique violation",
			dbErr:   testutils.PgError("23505"),
			wantErr: models.ErrDonationValidation,
		},
		{
			name:    "storage failure",
			dbErr:   technicalErr,
			wantErr: technicalErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := donationRequest()
			repo, pool := newRepository(t)

			pool.ExpectBeginTx(txOptions)

			expectInsertDonation(pool, donationOrderID, donationPayerID, req).
				WillReturnError(tt.dbErr)

			testutils.ExpectTxRolledBack(pool)

			got, err := repo.CreateDonation(
				context.Background(),
				donationOrderID,
				donationPayerID,
				req,
			)

			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestDonationRepository_CreateDonation_InvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		paymentOrderID uuid.UUID
		donorID        uuid.UUID
		req            *dto.CreateDonationRequest
	}{
		{
			name:           "nil payment order ID",
			paymentOrderID: uuid.Nil,
			donorID:        donationPayerID,
			req:            donationRequest(),
		},
		{
			name:           "nil donor ID",
			paymentOrderID: donationOrderID,
			donorID:        uuid.Nil,
			req:            donationRequest(),
		},
		{
			name:           "nil request",
			paymentOrderID: donationOrderID,
			donorID:        donationPayerID,
			req:            nil,
		},
		{
			name:           "nil author ID",
			paymentOrderID: donationOrderID,
			donorID:        donationPayerID,
			req:            &dto.CreateDonationRequest{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, _ := newRepository(t)

			got, err := repo.CreateDonation(
				context.Background(),
				tt.paymentOrderID,
				tt.donorID,
				tt.req,
			)

			require.Nil(t, got)
			require.ErrorIs(t, err, models.ErrDonationValidation)
		})
	}
}

func TestDonationRepository_CreateDonation_BeginFailure(t *testing.T) {
	t.Parallel()

	repo, pool := newRepository(t)
	pool.ExpectBeginTx(txOptions).WillReturnError(technicalErr)

	got, err := repo.CreateDonation(
		context.Background(),
		donationOrderID,
		donationPayerID,
		donationRequest(),
	)

	require.Nil(t, got)
	require.ErrorIs(t, err, technicalErr)
}

func TestDonationRepository_MarkPaid(t *testing.T) {
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

func TestDonationRepository_MarkPaid_Error(t *testing.T) {
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
	call   func(repo *DonationRepository, userID uuid.UUID, cursor *dto.DonationCursor) ([]*models.Donation, error)
}

func listMethods() []listMethod {
	ctx := context.Background()

	return []listMethod{
		{
			name:   "GetReceivedDonations",
			query:  receivedQuery,
			userID: donationAuthorID,
			call: func(
				repo *DonationRepository,
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
				repo *DonationRepository,
				userID uuid.UUID,
				cursor *dto.DonationCursor,
			) ([]*models.Donation, error) {
				return repo.GetSentDonations(ctx, userID, cursor, 21)
			},
		},
	}
}

func TestDonationRepository_ListDonations_PaidFilter(t *testing.T) {
	t.Parallel()

	require.Contains(t, receivedQuery, "paid = TRUE")
	require.NotContains(t, sentQuery, "paid = TRUE")
}

func TestDonationRepository_ListDonations_CursorArgs(t *testing.T) {
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

func TestDonationRepository_ListDonations_Scan(t *testing.T) {
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

func TestDonationRepository_ListDonations_Errors(t *testing.T) {
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

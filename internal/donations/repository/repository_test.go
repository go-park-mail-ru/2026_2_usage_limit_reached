package donationrepository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var (
	donationPayerID   = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	donationAuthorID  = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	donationOrderID   = uuid.MustParse("00000000-0000-4000-8000-000000000301")
	donationAttemptID = uuid.MustParse("00000000-0000-4000-8000-000000000401")
	donationTime      = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	donationMessage   = "Спасибо за посты!"
	technicalErr      = errors.New("donation storage unavailable")
	txOptions         = pgx.TxOptions{IsoLevel: pgx.ReadCommitted}

	insertOrderQuery    = testutils.SQLInOrder("INSERT INTO payment_order", "'donation', 'manual'", "'pending'", "ON CONFLICT (request_key) DO NOTHING", "RETURNING id")
	insertDonationQuery = testutils.SQLInOrder("INSERT INTO donation (payment_order_id, author_id, message, is_anonymous)", "RETURNING created_at")
	insertAttemptQuery  = testutils.SQLInOrder("INSERT INTO payment_attempt (payment_order_id, provider, status)", "'mock', 'created'", "RETURNING id")
	existingOrderQuery  = testutils.SQLInOrder("FROM payment_order po", "JOIN donation d ON d.payment_order_id = po.id", "WHERE po.request_key = $1")
	receivedQuery       = testutils.SQLInOrder("FROM donation d", `JOIN "user" u ON u.id = po.payer_id`, "WHERE d.author_id = $1", "AND po.status = 'paid'", "(po.paid_at, po.id) < ($2::timestamptz, $3::uuid)", "ORDER BY po.paid_at DESC", "LIMIT $4")
	sentQuery           = testutils.SQLInOrder("FROM donation d", `JOIN "user" u ON u.id = d.author_id`, "WHERE po.payer_id = $1", "< ($2::timestamptz, $3::uuid)", "ORDER BY d.created_at DESC", "LIMIT $4")

	existingOrderColumns = []string{"id", "author_id", "message", "is_anonymous", "expected_amount_minor", "currency", "status", "created_at", "attempt_id"}
	donationListColumns  = []string{"payment_order_id", "message", "is_anonymous", "expected_amount_minor", "currency", "status", "created_or_paid_at", "username", "nickname", "avatar_key"}
)

func donationRequest() *dto.CreateDonationRequest {
	return &dto.CreateDonationRequest{
		AuthorID:    donationAuthorID,
		RequestKey:  donationPayerID.String() + ":00000000-0000-4000-8000-000000000501",
		Message:     &donationMessage,
		IsAnonymous: false,
		AmountMinor: 50000,
		Currency:    "RUB",
	}
}

func TestDonationRepository_CreateDonation_NewOrder(t *testing.T) {
	t.Parallel()

	req := donationRequest()
	repo, pool := newRepository(t)
	donationCreated(pool, req)
	pool.ExpectQuery(insertAttemptQuery).WithArgs(donationOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(donationAttemptID))
	testutils.ExpectTxCommitted(pool)

	got, err := repo.CreateDonation(context.Background(), donationPayerID, req)

	require.NoError(t, err)
	require.Equal(t, &dto.SentDonation{
		PaymentOrderID:   donationOrderID,
		PaymentAttemptID: &donationAttemptID,
		Message:          &donationMessage,
		AmountMinor:      50000,
		Currency:         "RUB",
		Status:           "pending",
		CreatedAt:        donationTime,
	}, got)
}

func TestDonationRepository_CreateDonation_RepeatedRequest(t *testing.T) {
	t.Parallel()

	req := donationRequest()
	repo, pool := newRepository(t)
	pool.ExpectBeginTx(txOptions)
	expectInsertOrder(pool, req).WillReturnRows(pgxmock.NewRows([]string{"id"}))
	pool.ExpectQuery(existingOrderQuery).WithArgs(req.RequestKey).WillReturnRows(
		pgxmock.NewRows(existingOrderColumns).AddRow(
			donationOrderID, donationAuthorID, &donationMessage, false, int64(50000), "RUB", "paid", donationTime, &donationAttemptID,
		),
	)
	testutils.ExpectTxCommitted(pool)

	got, err := repo.CreateDonation(context.Background(), donationPayerID, req)

	require.NoError(t, err)
	require.Equal(t, &dto.SentDonation{
		PaymentOrderID:   donationOrderID,
		PaymentAttemptID: &donationAttemptID,
		Message:          &donationMessage,
		AmountMinor:      50000,
		Currency:         "RUB",
		Status:           "paid",
		CreatedAt:        donationTime,
	}, got)
}

func TestDonationRepository_CreateDonation_RequestKeyReused(t *testing.T) {
	t.Parallel()

	otherMessage := "Другое сообщение"
	tests := []struct {
		name        string
		authorID    uuid.UUID
		message     *string
		isAnonymous bool
		amount      int64
		currency    string
	}{
		{name: "other author", authorID: uuid.New(), message: &donationMessage, amount: 50000, currency: "RUB"},
		{name: "other message", authorID: donationAuthorID, message: &otherMessage, amount: 50000, currency: "RUB"},
		{name: "message removed", authorID: donationAuthorID, message: nil, amount: 50000, currency: "RUB"},
		{name: "other anonymity", authorID: donationAuthorID, message: &donationMessage, isAnonymous: true, amount: 50000, currency: "RUB"},
		{name: "other amount", authorID: donationAuthorID, message: &donationMessage, amount: 100, currency: "RUB"},
		{name: "other currency", authorID: donationAuthorID, message: &donationMessage, amount: 50000, currency: "USD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := donationRequest()
			repo, pool := newRepository(t)
			pool.ExpectBeginTx(txOptions)
			expectInsertOrder(pool, req).WillReturnRows(pgxmock.NewRows([]string{"id"}))
			pool.ExpectQuery(existingOrderQuery).WithArgs(req.RequestKey).WillReturnRows(
				pgxmock.NewRows(existingOrderColumns).AddRow(
					donationOrderID, tt.authorID, tt.message, tt.isAnonymous, tt.amount, tt.currency, "pending", donationTime, &donationAttemptID,
				),
			)
			testutils.ExpectTxRolledBack(pool)

			got, err := repo.CreateDonation(context.Background(), donationPayerID, req)

			require.Nil(t, got)
			require.ErrorIs(t, err, models.ErrIdempotencyKeyReused)
		})
	}
}

func TestDonationRepository_CreateDonation_ExistingOrderErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rows    *pgxmock.Rows
		dbErr   error
		wantErr error
	}{
		{name: "order without donation", rows: pgxmock.NewRows(existingOrderColumns), wantErr: models.ErrDonationNotFound},
		{name: "storage failure", dbErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := donationRequest()
			repo, pool := newRepository(t)
			pool.ExpectBeginTx(txOptions)
			expectInsertOrder(pool, req).WillReturnRows(pgxmock.NewRows([]string{"id"}))
			existing := pool.ExpectQuery(existingOrderQuery).WithArgs(req.RequestKey)
			if tt.rows != nil {
				existing.WillReturnRows(tt.rows)
			} else {
				existing.WillReturnError(tt.dbErr)
			}
			testutils.ExpectTxRolledBack(pool)

			got, err := repo.CreateDonation(context.Background(), donationPayerID, req)

			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestDonationRepository_CreateDonation_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expect  func(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest)
		wantErr error
	}{
		{
			name: "unknown payer",
			expect: func(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) {
				pool.ExpectBeginTx(txOptions)
				expectInsertOrder(pool, req).WillReturnError(testutils.PgError("23503"))
			},
			wantErr: models.ErrDonationValidation,
		},
		{
			name: "order violates check constraint",
			expect: func(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) {
				pool.ExpectBeginTx(txOptions)
				expectInsertOrder(pool, req).WillReturnError(testutils.PgError("23514"))
			},
			wantErr: models.ErrDonationValidation,
		},
		{
			name: "order storage failure",
			expect: func(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) {
				pool.ExpectBeginTx(txOptions)
				expectInsertOrder(pool, req).WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
		{
			name: "unknown author",
			expect: func(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) {
				orderCreated(pool, req)
				expectInsertDonation(pool, req).WillReturnError(testutils.PgError("23503"))
			},
			wantErr: models.ErrAuthorNotFound,
		},
		{
			name: "donation violates check constraint",
			expect: func(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) {
				orderCreated(pool, req)
				expectInsertDonation(pool, req).WillReturnError(testutils.PgError("23514"))
			},
			wantErr: models.ErrDonationValidation,
		},
		{
			name: "donation storage failure",
			expect: func(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) {
				orderCreated(pool, req)
				expectInsertDonation(pool, req).WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
		{
			name: "attempt violates check constraint",
			expect: func(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) {
				donationCreated(pool, req)
				pool.ExpectQuery(insertAttemptQuery).WithArgs(donationOrderID).WillReturnError(testutils.PgError("23514"))
			},
			wantErr: models.ErrDonationValidation,
		},
		{
			name: "attempt storage failure",
			expect: func(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) {
				donationCreated(pool, req)
				pool.ExpectQuery(insertAttemptQuery).WithArgs(donationOrderID).WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := donationRequest()
			repo, pool := newRepository(t)
			tt.expect(pool, req)
			testutils.ExpectTxRolledBack(pool)

			got, err := repo.CreateDonation(context.Background(), donationPayerID, req)

			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestDonationRepository_CreateDonation_BeginFailure(t *testing.T) {
	t.Parallel()

	repo, pool := newRepository(t)
	pool.ExpectBeginTx(txOptions).WillReturnError(technicalErr)

	got, err := repo.CreateDonation(context.Background(), donationPayerID, donationRequest())

	require.Nil(t, got)
	require.ErrorIs(t, err, technicalErr)
}

type listMethod struct {
	name  string
	query string
	row   []any
	call  func(r *DonationRepository, userID uuid.UUID, cursor *dto.DonationCursor) ([]*dto.GetDonationResponse, error)
}

func listMethods() []listMethod {
	ctx := context.Background()
	return []listMethod{
		{
			name: "GetReceivedDonations", query: receivedQuery,
			row: []any{donationOrderID, (*string)(nil), true, int64(1), "RUB", "paid", donationTime, (*string)(nil), (*string)(nil), (*string)(nil)},
			call: func(r *DonationRepository, u uuid.UUID, c *dto.DonationCursor) ([]*dto.GetDonationResponse, error) {
				return r.GetReceivedDonations(ctx, u, c, 21)
			},
		},
		{
			name: "GetSentDonations", query: sentQuery,
			row: []any{donationOrderID, (*string)(nil), true, int64(1), "RUB", "paid", donationTime, "author", "Автор", (*string)(nil)},
			call: func(r *DonationRepository, u uuid.UUID, c *dto.DonationCursor) ([]*dto.GetDonationResponse, error) {
				return r.GetSentDonations(ctx, u, c, 21)
			},
		},
	}
}

func TestDonationRepository_ListDonations_CursorArgs(t *testing.T) {
	t.Parallel()

	cursor := &dto.DonationCursor{CreatedOrPaidAt: donationTime, DonationID: donationOrderID}
	tests := []struct {
		name   string
		cursor *dto.DonationCursor
		args   []any
	}{
		{name: "first page", args: []any{donationPayerID, (*time.Time)(nil), (*uuid.UUID)(nil), 21}},
		{name: "next page", cursor: cursor, args: []any{donationPayerID, &cursor.CreatedOrPaidAt, &cursor.DonationID, 21}},
	}
	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				repo, pool := newRepository(t)
				pool.ExpectQuery(m.query).WithArgs(tt.args...).WillReturnRows(pgxmock.NewRows(donationListColumns))

				got, err := m.call(repo, donationPayerID, tt.cursor)

				require.NoError(t, err)
				require.NotNil(t, got)
				require.Empty(t, got)
			})
		}
	}
}

func TestDonationRepository_GetReceivedDonations_DonorVisibility(t *testing.T) {
	t.Parallel()

	username, nickname, avatar := "ivan001", "Иван", "avatars/ivan.png"
	repo, pool := newRepository(t)
	pool.ExpectQuery(receivedQuery).WithArgs(donationAuthorID, (*time.Time)(nil), (*uuid.UUID)(nil), 21).WillReturnRows(
		pgxmock.NewRows(donationListColumns).
			AddRow(donationOrderID, &donationMessage, false, int64(50000), "RUB", "paid", donationTime, &username, &nickname, &avatar).
			AddRow(donationAttemptID, (*string)(nil), true, int64(100), "RUB", "paid", donationTime, (*string)(nil), (*string)(nil), (*string)(nil)),
	)

	got, err := repo.GetReceivedDonations(context.Background(), donationAuthorID, nil, 21)

	require.NoError(t, err)
	require.Equal(t, []*dto.GetDonationResponse{
		{
			PaymentOrderID: donationOrderID, Message: &donationMessage, AmountMinor: 50000, Currency: "RUB",
			Status: "paid", CreatedOrPaidAt: donationTime,
			User: &dto.DonationUserInfo{Username: "ivan001", Nickname: "Иван", AvatarUrl: &avatar},
		},
		{
			PaymentOrderID: donationAttemptID, IsAnonymous: true, AmountMinor: 100, Currency: "RUB",
			Status: "paid", CreatedOrPaidAt: donationTime,
		},
	}, got)
}

func TestDonationRepository_GetSentDonations_AuthorInfo(t *testing.T) {
	t.Parallel()

	repo, pool := newRepository(t)
	pool.ExpectQuery(sentQuery).WithArgs(donationPayerID, (*time.Time)(nil), (*uuid.UUID)(nil), 21).WillReturnRows(
		pgxmock.NewRows(donationListColumns).
			AddRow(donationOrderID, (*string)(nil), true, int64(50000), "RUB", "pending", donationTime, "author", "Автор", (*string)(nil)),
	)

	got, err := repo.GetSentDonations(context.Background(), donationPayerID, nil, 21)

	require.NoError(t, err)
	require.Equal(t, []*dto.GetDonationResponse{
		{
			PaymentOrderID: donationOrderID, IsAnonymous: true, AmountMinor: 50000, Currency: "RUB",
			Status: "pending", CreatedOrPaidAt: donationTime,
			User: &dto.DonationUserInfo{Username: "author", Nickname: "Автор"},
		},
	}, got)
}

func TestDonationRepository_ListDonations_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		expect func(q *pgxmock.ExpectedQuery, row []any)
	}{
		{name: "query failure", expect: func(q *pgxmock.ExpectedQuery, _ []any) { q.WillReturnError(technicalErr) }},
		{name: "row iteration failure", expect: func(q *pgxmock.ExpectedQuery, row []any) {
			q.WillReturnRows(pgxmock.NewRows(donationListColumns).AddRow(row...).RowError(0, technicalErr))
		}},
	}
	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				repo, pool := newRepository(t)
				tt.expect(pool.ExpectQuery(m.query).WithArgs(donationPayerID, (*time.Time)(nil), (*uuid.UUID)(nil), 21), m.row)

				got, err := m.call(repo, donationPayerID, nil)

				require.Empty(t, got)
				require.ErrorIs(t, err, technicalErr)
			})
		}

		t.Run(m.name+"/unexpected columns", func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)
			pool.ExpectQuery(m.query).WithArgs(donationPayerID, (*time.Time)(nil), (*uuid.UUID)(nil), 21).
				WillReturnRows(pgxmock.NewRows([]string{"payment_order_id"}).AddRow(donationOrderID))

			got, err := m.call(repo, donationPayerID, nil)

			require.Nil(t, got)
			require.Error(t, err)
		})
	}
}

package repository

import (
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v5"
)

func newRepository(t *testing.T) (*DonationRepository, pgxmock.PgxPoolIface) {
	t.Helper()

	pool := testutils.SetupPgxMock(t)

	return &DonationRepository{
		pool:   pool,
		logger: testutils.SetupLogger(),
	}, pool
}

func expectInsertDonation(pool pgxmock.PgxPoolIface, paymentOrderID, donorID uuid.UUID, req *dto.CreateDonationRequest) *pgxmock.ExpectedQuery {
	return pool.ExpectQuery(insertDonationQuery).
		WithArgs(
			paymentOrderID,
			donorID,
			req.AuthorID,
			req.Message,
			req.IsAnonymous,
		)
}

func donationCreated(pool pgxmock.PgxPoolIface, paymentOrderID, donorID uuid.UUID, req *dto.CreateDonationRequest) {
	pool.ExpectBeginTx(txOptions)

	expectInsertDonation(pool, paymentOrderID, donorID, req).
		WillReturnRows(
			pgxmock.NewRows([]string{"created_at", "updated_at"}).
				AddRow(donationTime, donationUpdatedAt),
		)
}

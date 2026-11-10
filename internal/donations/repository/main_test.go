package donationrepository

import (
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/pashagolub/pgxmock/v5"
)

func newRepository(t *testing.T) (*DonationRepository, pgxmock.PgxPoolIface) {
	t.Helper()

	pool := testutils.SetupPgxMock(t)
	return &DonationRepository{pool: pool, logger: testutils.SetupLogger()}, pool
}

func expectInsertOrder(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) *pgxmock.ExpectedQuery {
	return pool.ExpectQuery(insertOrderQuery).WithArgs(donationPayerID, req.RequestKey, req.AmountMinor, req.Currency)
}

func expectInsertDonation(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) *pgxmock.ExpectedQuery {
	return pool.ExpectQuery(insertDonationQuery).WithArgs(donationOrderID, req.AuthorID, req.Message, req.IsAnonymous)
}

func orderCreated(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) {
	pool.ExpectBeginTx(txOptions)
	expectInsertOrder(pool, req).WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(donationOrderID))
}

func donationCreated(pool pgxmock.PgxPoolIface, req *dto.CreateDonationRequest) {
	orderCreated(pool, req)
	expectInsertDonation(pool, req).WillReturnRows(pgxmock.NewRows([]string{"created_at"}).AddRow(donationTime))
}

package repository

import (
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/pashagolub/pgxmock/v5"
)

func newRepository(t *testing.T) (*MonetizationRepository, pgxmock.PgxPoolIface) {
	t.Helper()

	pool := testutils.SetupPgxMock(t)

	return &MonetizationRepository{
		pool:   pool,
		logger: testutils.SetupLogger(),
	}, pool
}

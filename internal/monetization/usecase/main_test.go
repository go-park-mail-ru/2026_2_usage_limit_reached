package usecase_test

import (
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	donationusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase/mock_monetization_usecase"
	mock_payments_usecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase/mock_monetization_usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider/mock_pkg_payment_provider"
	"github.com/google/uuid"
)

func newUsecase(t *testing.T) (
	*donationusecase.MonetizationUsecase,
	*mock_monetization_usecase.MockMonetizationRepository,
	*mock_payments_usecase.MockPaymentRepository,
	*mock_pkg_payment_provider.MockProvider,
	*mock_monetization_usecase.MockIdentityClient,
) {
	return newUsecaseWithReader(t, defaultOnlineConfReader())
}

func newUsecaseWithReader(t *testing.T, reader donationusecase.OnlineConfClient) (
	*donationusecase.MonetizationUsecase,
	*mock_monetization_usecase.MockMonetizationRepository,
	*mock_payments_usecase.MockPaymentRepository,
	*mock_pkg_payment_provider.MockProvider,
	*mock_monetization_usecase.MockIdentityClient,
) {
	t.Helper()

	repo := mock_monetization_usecase.NewMockMonetizationRepository(t)
	payments := mock_payments_usecase.NewMockPaymentRepository(t)
	provider := mock_pkg_payment_provider.NewMockProvider(t)
	identity := mock_monetization_usecase.NewMockIdentityClient(t)

	return donationusecase.New(repo, payments, provider, identity, reader), repo, payments, provider, identity
}

type testOnlineConfReader struct {
	values map[string]string
}

func (r testOnlineConfReader) Value(scope, key string) (string, bool) {
	value, ok := r.values[scope+"/"+key]
	return value, ok
}

var _ donationusecase.OnlineConfClient = testOnlineConfReader{}

func defaultOnlineConfReader() testOnlineConfReader {
	return testOnlineConfReader{values: map[string]string{"front/payment.min_amount_minor": "100"}}
}

func donations(n int) []*models.Donation {
	items := make([]*models.Donation, n)

	for i := range items {
		createdAt := donationTime.Add(-time.Duration(i) * time.Hour)

		items[i] = &models.Donation{
			PaymentOrderID: uuid.New(),
			DonorID:        donationPayerID,
			AuthorID:       donationAuthorID,
			CreatedAt:      createdAt,
			UpdatedAt:      createdAt,
		}
	}

	return items
}

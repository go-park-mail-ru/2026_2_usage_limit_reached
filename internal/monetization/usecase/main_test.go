package usecase_test

import (
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	donationusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase/mock_monetization_usecase"
	"github.com/google/uuid"
)

func newUsecase(t *testing.T) (
	*donationusecase.MonetizationUsecase,
	*mock_monetization_usecase.MockDonationRepository,
	*mock_monetization_usecase.MockPaymentProcessor,
	*mock_monetization_usecase.MockIdentityClient,
) {
	t.Helper()

	repo := mock_monetization_usecase.NewMockDonationRepository(t)
	payments := mock_monetization_usecase.NewMockPaymentProcessor(t)
	identity := mock_monetization_usecase.NewMockIdentityClient(t)

	return donationusecase.New(repo, payments, identity), repo, payments, identity
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

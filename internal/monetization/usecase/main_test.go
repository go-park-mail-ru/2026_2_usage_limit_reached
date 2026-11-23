package usecase_test

import (
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	donationusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase/mock_monetization_usecase"
	"github.com/google/uuid"
)

func newUsecase(t *testing.T) (*donationusecase.DonationUsecase, *mock_monetization_usecase.MockDonationRepository, *mock_monetization_usecase.MockPaymentProcessor) {
	t.Helper()

	repo := mock_monetization_usecase.NewMockDonationRepository(t)
	payments := mock_monetization_usecase.NewMockPaymentProcessor(t)
	return donationusecase.New(repo, payments), repo, payments
}

func donations(n int) []*dto.GetDonationResponse {
	items := make([]*dto.GetDonationResponse, n)
	for i := range items {
		items[i] = &dto.GetDonationResponse{
			PaymentOrderID:  uuid.New(),
			AmountMinor:     50000,
			Currency:        "RUB",
			Status:          "paid",
			CreatedOrPaidAt: donationTime.Add(-time.Duration(i) * time.Hour),
		}
	}
	return items
}

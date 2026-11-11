package donationusecase_test

import (
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	donationusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/usecase/mock_donations_usecase"
	"github.com/google/uuid"
)

func newUsecase(t *testing.T) (*donationusecase.DonationUsecase, *mock_donations_usecase.MockDonationRepository, *mock_donations_usecase.MockPaymentProcessor) {
	t.Helper()

	repo := mock_donations_usecase.NewMockDonationRepository(t)
	payments := mock_donations_usecase.NewMockPaymentProcessor(t)
	return donationusecase.NewDonationUsecase(repo, payments), repo, payments
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

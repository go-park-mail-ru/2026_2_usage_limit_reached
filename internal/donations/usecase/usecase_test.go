package donationusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/models"
	donationusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/usecase/mock_donations_usecase"
	paymentmodels "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	paymentsusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const pageLimit = 2

var (
	donationPayerID   = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	donationAuthorID  = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	donationOrderID   = uuid.MustParse("00000000-0000-4000-8000-000000000301")
	donationAttemptID = uuid.MustParse("00000000-0000-4000-8000-000000000401")
	donationTime      = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	donationMessage   = "Спасибо за посты!"
	technicalErr      = errors.New("donation storage unavailable")
	donationCursor    = &dto.DonationCursor{CreatedOrPaidAt: donationTime, DonationID: donationOrderID}

	donationReq = dto.CreateDonationRequest{
		AuthorID:    donationAuthorID,
		RequestKey:  "00000000-0000-4000-8000-000000000501",
		Message:     &donationMessage,
		AmountMinor: 50000,
		Currency:    "RUB",
	}
	storedReq = &dto.CreateDonationRequest{
		AuthorID:    donationAuthorID,
		RequestKey:  donationPayerID.String() + ":00000000-0000-4000-8000-000000000501",
		Message:     &donationMessage,
		AmountMinor: 50000,
		Currency:    "RUB",
	}
)

func sentDonation(status paymentmodels.PaymentOrderStatus, attemptID *uuid.UUID) *dto.SentDonation {
	return &dto.SentDonation{
		PaymentOrderID:   donationOrderID,
		PaymentAttemptID: attemptID,
		Message:          &donationMessage,
		AmountMinor:      50000,
		Currency:         "RUB",
		Status:           string(status),
		CreatedAt:        donationTime,
	}
}

func TestDonationUsecase_Donate_PaysPendingOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		payStatus paymentmodels.PaymentOrderStatus
	}{
		{name: "payment succeeded", payStatus: paymentmodels.StatusPaid},
		{name: "payment failed", payStatus: paymentmodels.StatusFailed},
		{name: "payment still processing", payStatus: paymentmodels.StatusProcessing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, repo, payments := newUsecase(t)
			mock.InOrder(
				repo.EXPECT().CreateDonation(mock.Anything, donationPayerID, storedReq).
					Return(sentDonation(paymentmodels.StatusPending, &donationAttemptID), nil).Call,
				payments.EXPECT().Pay(mock.Anything, donationOrderID, donationAttemptID).Return(tt.payStatus, nil).Call,
			)

			got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

			require.NoError(t, err)
			require.Equal(t, sentDonation(tt.payStatus, &donationAttemptID), got)
		})
	}
}

func TestDonationUsecase_Donate_SkipsPayment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		donation *dto.SentDonation
	}{
		{name: "repeated request for paid order", donation: sentDonation(paymentmodels.StatusPaid, &donationAttemptID)},
		{name: "repeated request for failed order", donation: sentDonation(paymentmodels.StatusFailed, &donationAttemptID)},
		{name: "repeated request for canceled order", donation: sentDonation(paymentmodels.StatusCanceled, &donationAttemptID)},
		{name: "pending order without attempt", donation: sentDonation(paymentmodels.StatusPending, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, repo, _ := newUsecase(t)
			repo.EXPECT().CreateDonation(mock.Anything, donationPayerID, storedReq).Return(tt.donation, nil)

			got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

			require.NoError(t, err)
			require.Same(t, tt.donation, got)
		})
	}
}

func TestDonationUsecase_Donate_SelfDonation(t *testing.T) {
	t.Parallel()

	uc, _, _ := newUsecase(t)
	req := donationReq
	req.AuthorID = donationPayerID

	got, err := uc.Donate(context.Background(), donationPayerID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, donationusecase.ErrForbidden)
}

func TestDonationUsecase_Donate_RepositoryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{name: "unknown donation", repoErr: models.ErrDonationNotFound, wantErr: donationusecase.ErrNotFound},
		{name: "unknown author", repoErr: models.ErrAuthorNotFound, wantErr: donationusecase.ErrNotFound},
		{name: "self donation", repoErr: models.ErrSelfDonation, wantErr: donationusecase.ErrForbidden},
		{name: "request key reused", repoErr: models.ErrIdempotencyKeyReused, wantErr: donationusecase.ErrConflict},
		{name: "invalid donation", repoErr: models.ErrDonationValidation, wantErr: donationusecase.ErrValidation},
		{name: "storage failure", repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, repo, _ := newUsecase(t)
			repo.EXPECT().CreateDonation(mock.Anything, donationPayerID, storedReq).Return(nil, tt.repoErr)

			got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestDonationUsecase_Donate_PaymentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payErr  error
		wantErr error
	}{
		{name: "request key reused in payments", payErr: paymentsusecase.ErrIdempotencyKeyReused, wantErr: donationusecase.ErrConflict},
		{name: "payment failure", payErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, repo, payments := newUsecase(t)
			repo.EXPECT().CreateDonation(mock.Anything, donationPayerID, storedReq).
				Return(sentDonation(paymentmodels.StatusPending, &donationAttemptID), nil)
			payments.EXPECT().Pay(mock.Anything, donationOrderID, donationAttemptID).Return("", tt.payErr)

			got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr != donationusecase.ErrConflict {
				require.False(t, errors.Is(err, donationusecase.ErrConflict))
			}
		})
	}
}

type listMethod struct {
	name   string
	expect func(repo *mock_donations_usecase.MockDonationRepository, items []*dto.GetDonationResponse, err error)
	call   func(uc *donationusecase.DonationUsecase) (*dto.DonationPage, error)
}

func listMethods() []listMethod {
	ctx := context.Background()
	return []listMethod{
		{
			name: "GetSentDonations",
			expect: func(repo *mock_donations_usecase.MockDonationRepository, items []*dto.GetDonationResponse, err error) {
				repo.EXPECT().GetSentDonations(mock.Anything, donationPayerID, donationCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *donationusecase.DonationUsecase) (*dto.DonationPage, error) {
				return uc.GetSentDonations(ctx, donationPayerID, donationCursor, pageLimit)
			},
		},
		{
			name: "GetReceivedDonations",
			expect: func(repo *mock_donations_usecase.MockDonationRepository, items []*dto.GetDonationResponse, err error) {
				repo.EXPECT().GetReceivedDonations(mock.Anything, donationPayerID, donationCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *donationusecase.DonationUsecase) (*dto.DonationPage, error) {
				return uc.GetReceivedDonations(ctx, donationPayerID, donationCursor, pageLimit)
			},
		},
	}
}

func TestDonationUsecase_ListDonations_Pagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fetched   int
		wantItems int
		wantNext  bool
	}{
		{name: "empty page", fetched: 0, wantItems: 0},
		{name: "last full page", fetched: pageLimit, wantItems: pageLimit},
		{name: "extra item becomes next cursor", fetched: pageLimit + 1, wantItems: pageLimit, wantNext: true},
	}
	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				items := donations(tt.fetched)
				uc, repo, _ := newUsecase(t)
				m.expect(repo, items, nil)

				got, err := m.call(uc)

				require.NoError(t, err)
				require.Equal(t, items[:tt.wantItems], got.Items)
				if !tt.wantNext {
					require.Nil(t, got.Next)

					return
				}
				last := items[tt.wantItems-1]
				require.Equal(t, &dto.DonationCursor{CreatedOrPaidAt: last.CreatedOrPaidAt, DonationID: last.PaymentOrderID}, got.Next)
			})
		}
	}
}

func TestDonationUsecase_ListDonations_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{name: "unknown user", repoErr: models.ErrAuthorNotFound, wantErr: donationusecase.ErrNotFound},
		{name: "storage failure", repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				uc, repo, _ := newUsecase(t)
				m.expect(repo, nil, tt.repoErr)

				got, err := m.call(uc)

				require.Nil(t, got)
				require.ErrorIs(t, err, tt.wantErr)
			})
		}
	}
}

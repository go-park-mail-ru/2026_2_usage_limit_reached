package donationusecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/models"
	paymentmodels "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	providermock "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/provider/mock"
	"github.com/google/uuid"
)

var (
	ErrValidation   = errors.New("validation error")
	ErrNotFound     = errors.New("not found")
	ErrSelfDonation = errors.New("forbidden to donate to yourself")
)

type PaymentProcessor interface {
	Pay(ctx context.Context, orderID, attemptID uuid.UUID) (paymentmodels.PaymentOrderStatus, error)
}

type DonationRepository interface {
	CreateDonation(ctx context.Context, payerID uuid.UUID, req *dto.CreateDonationRequest) (*dto.SentDonation, error)
	GetReceivedDonations(ctx context.Context, authorID uuid.UUID, limit int) ([]*dto.ReceivedDonation, error)
	GetSentDonations(ctx context.Context, payerID uuid.UUID, limit int) ([]*dto.SentDonation, error)
}

var repoErrors = map[error]error{
	models.ErrDonationNotFound:   ErrNotFound,
	models.ErrAuthorNotFound:     ErrNotFound,
	models.ErrSelfDonation:       ErrSelfDonation,
	models.ErrDonationValidation: ErrValidation,
}

func mapRepoError(op string, err error) error {
	to, ok := repoErrors[err]
	if ok {
		return fmt.Errorf("%s: %w", op, to)
	}
	return fmt.Errorf("%s: %w", op, err)
}

type DonationUsecase struct {
	repo     DonationRepository
	provider providermock.Provider
	payments PaymentProcessor
}

func (uc *DonationUsecase) Donate(ctx context.Context, payerID uuid.UUID, donationReq dto.CreateDonationRequest) (*dto.SentDonation, error) {
	if payerID == donationReq.AuthorID {
		return nil, ErrSelfDonation
	}

	d, err := uc.repo.CreateDonation(ctx, payerID, &dto.CreateDonationRequest{
		RequestKey:  donationReq.RequestKey,
		AuthorID:    donationReq.AuthorID,
		AmountMinor: donationReq.AmountMinor,
		Currency:    donationReq.Currency,
		Message:     donationReq.Message,
		IsAnonymous: donationReq.IsAnonymous,
	})
	if err != nil {
		return nil, mapRepoError("create donation", err)
	}

	if d.Status != string(paymentmodels.StatusPending) || d.PaymentAttemptID == nil {
		return d, nil
	}

	status, err := uc.payments.Pay(ctx, d.PaymentOrderID, *d.PaymentAttemptID)
	if err != nil {
		return nil, fmt.Errorf("pay: %w", err)
	}
	d.Status = string(status)
	return d, nil
}

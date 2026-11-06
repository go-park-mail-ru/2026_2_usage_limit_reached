package donationusecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/models"
	paymentmodels "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	paymentsusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/usecase"
	"github.com/google/uuid"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("payment conflict")
)

type PaymentProcessor interface {
	Pay(ctx context.Context, orderID, attemptID uuid.UUID) (paymentmodels.PaymentOrderStatus, error)
}

type DonationRepository interface {
	CreateDonation(ctx context.Context, payerID uuid.UUID, req *dto.CreateDonationRequest) (*dto.SentDonation, error)
	GetReceivedDonations(ctx context.Context, authorID uuid.UUID, cursor *dto.DonationCursor, limit int) ([]*dto.GetDonationResponse, error)
	GetSentDonations(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) ([]*dto.GetDonationResponse, error)
}

var repoErrors = []struct{ from, to error }{
	{models.ErrDonationNotFound, ErrNotFound},
	{models.ErrAuthorNotFound, ErrNotFound},
	{models.ErrSelfDonation, ErrForbidden},
	{models.ErrIdempotencyKeyReused, ErrConflict},
	{models.ErrDonationValidation, ErrValidation},
}

func mapRepoError(op string, err error) error {
	for _, e := range repoErrors {
		if errors.Is(err, e.from) {
			return fmt.Errorf("%s: %w", op, e.to)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

type DonationUsecase struct {
	repo     DonationRepository
	payments PaymentProcessor
}

func NewDonationUsecase(repo DonationRepository, payments PaymentProcessor) *DonationUsecase {
	return &DonationUsecase{repo: repo, payments: payments}
}

func (uc *DonationUsecase) Donate(ctx context.Context, payerID uuid.UUID, donationReq dto.CreateDonationRequest) (*dto.SentDonation, error) {
	if payerID == donationReq.AuthorID {
		return nil, ErrForbidden
	}

	reqKey := payerID.String() + ":" + donationReq.RequestKey

	d, err := uc.repo.CreateDonation(ctx, payerID, &dto.CreateDonationRequest{
		RequestKey:  reqKey,
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
		if errors.Is(err, paymentsusecase.ErrIdempotencyKeyReused) {
			return nil, fmt.Errorf("%w: %w", ErrConflict, err)
		}
		return nil, fmt.Errorf("pay: %w", err)
	}
	d.Status = string(status)
	return d, nil
}

func (uc *DonationUsecase) GetSentDonations(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error) {
	donations, err := uc.repo.GetSentDonations(ctx, payerID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get sent donations", err)
	}
	return paginate(donations, limit), nil
}

func (uc *DonationUsecase) GetReceivedDonations(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error) {
	donations, err := uc.repo.GetReceivedDonations(ctx, payerID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get received donations", err)
	}
	return paginate(donations, limit), nil
}

func paginate(items []*dto.GetDonationResponse, limit int) *dto.DonationPage {
	if len(items) <= limit {
		return &dto.DonationPage{Items: items, NextCursor: nil}
	}
	items = items[:limit]
	last := items[limit-1]
	next := dto.DonationCursor{CreatedOrPaidAt: last.CreatedOrPaidAt, DonationID: last.PaymentOrderID}
	return &dto.DonationPage{Items: items, NextCursor: &next}
}

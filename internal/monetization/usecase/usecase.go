package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/google/uuid"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("payment conflict")
)

const statusPending = "pending"

type PaymentProcessor interface {
	Pay(ctx context.Context, orderID uuid.UUID) (string, error)
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

func New(repo DonationRepository, payments PaymentProcessor) *DonationUsecase {
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

	if d.Status != statusPending {
		return d, nil
	}

	if d.PaymentOrderID == uuid.Nil {
		return nil, errors.New("pending donation has no payment order")
	}

	paymentStatus, err := uc.payments.Pay(ctx, d.PaymentOrderID)
	if err != nil {
		return nil, fmt.Errorf("pay: %w", err)
	}

	d.Status = paymentStatus
	return d, nil
}

func (uc *DonationUsecase) GetSentDonations(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error) {
	donations, err := uc.repo.GetSentDonations(ctx, payerID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get sent donations", err)
	}
	page := feed.Paginate(donations, limit, func(d *dto.GetDonationResponse) dto.DonationCursor {
		return dto.DonationCursor{CreatedOrPaidAt: d.CreatedOrPaidAt, DonationID: d.PaymentOrderID}
	})
	return (*dto.DonationPage)(page), nil
}

func (uc *DonationUsecase) GetReceivedDonations(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error) {
	donations, err := uc.repo.GetReceivedDonations(ctx, payerID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get received donations", err)
	}
	page := feed.Paginate(donations, limit, func(d *dto.GetDonationResponse) dto.DonationCursor {
		return dto.DonationCursor{CreatedOrPaidAt: d.CreatedOrPaidAt, DonationID: d.PaymentOrderID}
	})
	return (*dto.DonationPage)(page), nil
}

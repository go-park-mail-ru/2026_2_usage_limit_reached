package usecase

import (
	"context"
	"errors"
	"fmt"

	identitycontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/identity"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	paymentProvider "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/google/uuid"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("payment conflict")
)

type IdentityClient interface {
	GetUsers(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*identitycontracts.UserInfo, error)
}

type DonationRepository interface {
	CreateDonationWithPayment(ctx context.Context, req dto.CreateDonationWithPayment) (*models.Donation, *models.PaymentOrder, error)
	GetReceivedDonations(ctx context.Context, authorID uuid.UUID, cursor *dto.DonationCursor, limit int) ([]*models.Donation, error)
	GetSentDonations(ctx context.Context, donorID uuid.UUID, cursor *dto.DonationCursor, limit int) ([]*models.Donation, error)
	MarkPaid(ctx context.Context, paymentOrderID uuid.UUID) error
}

var repoErrors = []struct{ from, to error }{
	{models.ErrDonationNotFound, ErrNotFound},
	{models.ErrAuthorNotFound, ErrNotFound},
	{models.ErrSelfDonation, ErrForbidden},
	{models.ErrIdempotencyKeyReused, ErrConflict},
	{models.ErrDonationValidation, ErrValidation},
	{models.ErrPaymentOrderNotFound, ErrNotFound},
	{models.ErrPaymentAttemptNotFound, ErrNotFound},
	{models.ErrForbiddenUpdate, ErrForbidden},
	{models.ErrPaymentOrderValidation, ErrValidation},
	{models.ErrPaymentAttemptValidation, ErrValidation},
}

func mapRepoError(op string, err error) error {
	for _, e := range repoErrors {
		if errors.Is(err, e.from) {
			return fmt.Errorf("%s: %w: %w", op, e.to, err)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

type MonetizationUsecase struct {
	repo            DonationRepository
	paymentRepo     PaymentRepo
	paymentProvider paymentProvider.Provider
	identity        IdentityClient
	validator       *validator.Validator
}

func New(repo DonationRepository, paymentRepo PaymentRepo, provider paymentProvider.Provider, identity IdentityClient, v *validator.Validator) *MonetizationUsecase {
	return &MonetizationUsecase{
		repo:            repo,
		paymentRepo:     paymentRepo,
		paymentProvider: provider,
		identity:        identity,
		validator:       v,
	}
}

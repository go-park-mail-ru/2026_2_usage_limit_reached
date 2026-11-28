package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	identitycontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/identity"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	paymentProvider "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider"
	"github.com/google/uuid"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("payment conflict")
)

const (
	defaultDonationMinAmountMinor int64 = 100
	donationMinimumScope                = "front"
	donationMinimumKey                  = "payment.min_amount_minor"
)

type IdentityClient interface {
	GetUsers(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*identitycontracts.UserInfo, error)
}

type OnlineConfClient interface {
	Value(scope, key string) (string, bool)
}

type MonetizationRepository interface {
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
	repo            MonetizationRepository
	paymentRepo     PaymentRepository
	paymentProvider paymentProvider.Provider
	identity        IdentityClient
	onlineconf      OnlineConfClient
}

func New(repo MonetizationRepository, paymentRepo PaymentRepository, provider paymentProvider.Provider, identity IdentityClient, onlineconf OnlineConfClient) *MonetizationUsecase {
	return &MonetizationUsecase{
		repo:            repo,
		paymentRepo:     paymentRepo,
		paymentProvider: provider,
		identity:        identity,
		onlineconf:      onlineconf,
	}
}

func (uc *MonetizationUsecase) donationMinimum() (int64, error) {
	minimum := defaultDonationMinAmountMinor
	if uc.onlineconf != nil {
		if raw, ok := uc.onlineconf.Value(donationMinimumScope, donationMinimumKey); ok {
			if err := json.Unmarshal([]byte(raw), &minimum); err != nil {
				return 0, fmt.Errorf("parse minimum donation amount: %w", err)
			}
		}
	}

	if minimum <= 0 || minimum%100 != 0 {
		return 0, fmt.Errorf("invalid minimum donation amount: %d", minimum)
	}

	return minimum, nil
}

func (uc *MonetizationUsecase) GetDonationSettings(context.Context) (*dto.DonationSettings, error) {
	minimum, err := uc.donationMinimum()
	if err != nil {
		return nil, err
	}

	return &dto.DonationSettings{MinAmountMinor: minimum}, nil
}

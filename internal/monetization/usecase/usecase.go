package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	identitycontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/identity"
	paymentcontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/payment"
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

const (
	paymentStatusPaid      = "paid"
	paymentPurposeDonation = "donation"
	paymentOriginManual    = "manual"
)

type PaymentProcessor interface {
	CreatePayment(ctx context.Context, req paymentcontracts.CreatePaymentRequest) (*paymentcontracts.PaymentOrder, error)
	GetPayments(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]*paymentcontracts.PaymentOrder, error)
	Pay(ctx context.Context, orderID uuid.UUID) (string, error)
}

type IdentityClient interface {
	GetUsers(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*identitycontracts.UserInfo, error)
}

type DonationRepository interface {
	CreateDonation(ctx context.Context, paymentOrderID, donorID uuid.UUID, req *dto.CreateDonationRequest) (*models.Donation, error)
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
}

func mapRepoError(op string, err error) error {
	for _, e := range repoErrors {
		if errors.Is(err, e.from) {
			return fmt.Errorf("%s: %w", op, e.to)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

type MonetizationUsecase struct {
	repo     DonationRepository
	payment  PaymentProcessor
	identity IdentityClient
}

func New(repo DonationRepository, payments PaymentProcessor, identity IdentityClient) *MonetizationUsecase {
	return &MonetizationUsecase{
		repo:     repo,
		payment:  payments,
		identity: identity,
	}
}

func (uc *MonetizationUsecase) Donate(ctx context.Context, donorID uuid.UUID, donationReq dto.CreateDonationRequest) (*dto.SentDonation, error) {
	if donorID == uuid.Nil || donationReq.AuthorID == uuid.Nil {
		return nil, ErrValidation
	}

	if donorID == donationReq.AuthorID {
		return nil, ErrForbidden
	}

	users, err := uc.identity.GetUsers(ctx, []uuid.UUID{donationReq.AuthorID})
	if err != nil {
		return nil, fmt.Errorf("get donation author: %w", err)
	}

	author, ok := users[donationReq.AuthorID]
	if !ok || author == nil {
		return nil, ErrNotFound
	}

	expiresAt := time.Now().Add(15 * time.Minute)
	order, err := uc.payment.CreatePayment(ctx, paymentcontracts.CreatePaymentRequest{
		PayerID:     donorID,
		RequestKey:  donationReq.RequestKey,
		Purpose:     paymentPurposeDonation,
		Origin:      paymentOriginManual,
		AmountMinor: donationReq.AmountMinor,
		Currency:    donationReq.Currency,
		ExpiresAt:   &expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create payment order: %w", err)
	}

	if order == nil || order.ID == uuid.Nil {
		return nil, errors.New("payments returned an invalid payment order")
	}

	donation, err := uc.repo.CreateDonation(
		ctx,
		order.ID,
		donorID,
		&dto.CreateDonationRequest{
			AuthorID:    donationReq.AuthorID,
			Message:     donationReq.Message,
			IsAnonymous: donationReq.IsAnonymous,
		},
	)
	if err != nil {
		return nil, mapRepoError("create donation", err)
	}

	paymentStatus, err := uc.payment.Pay(ctx, donation.PaymentOrderID)
	if err != nil {
		return nil, fmt.Errorf("pay donation: %w", err)
	}

	if paymentStatus == paymentStatusPaid {
		if err := uc.repo.MarkPaid(ctx, order.ID); err != nil {
			return nil, err
		}
	}

	return &dto.SentDonation{
		PaymentOrderID: donation.PaymentOrderID,
		Message:        donation.Message,
		IsAnonymous:    donation.IsAnonymous,
		AmountMinor:    order.ExpectedAmountMinor,
		Currency:       order.Currency,
		Status:         paymentStatus,
		CreatedAt:      donation.CreatedAt,
		Author:         toDonationUserInfo(author),
	}, nil
}

func (uc *MonetizationUsecase) GetSentDonations(ctx context.Context, donorID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error) {
	donations, err := uc.repo.GetSentDonations(ctx, donorID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get sent donations", err)
	}

	return uc.buildDonationPage(ctx, donations, limit, false)
}

func (uc *MonetizationUsecase) GetReceivedDonations(ctx context.Context, authorID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error) {
	donations, err := uc.repo.GetReceivedDonations(ctx, authorID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get received donations", err)
	}

	return uc.buildDonationPage(ctx, donations, limit, true)
}

func (uc *MonetizationUsecase) buildDonationPage(ctx context.Context, donations []*models.Donation, limit int, received bool) (*dto.DonationPage, error) {
	page := feed.Paginate(
		donations,
		limit,
		func(d *models.Donation) dto.DonationCursor {
			return dto.DonationCursor{
				CreatedAt:  d.CreatedAt,
				DonationID: d.PaymentOrderID,
			}
		},
	)

	result := &dto.DonationPage{
		Items: make([]*dto.GetDonationResponse, 0, len(page.Items)),
		Next:  page.Next,
	}

	if len(page.Items) == 0 {
		return result, nil
	}

	orderIDs := make([]uuid.UUID, 0, len(page.Items))
	userIDs := make([]uuid.UUID, 0, len(page.Items))

	for _, donation := range page.Items {
		orderIDs = append(orderIDs, donation.PaymentOrderID)

		if received {
			if !donation.IsAnonymous {
				userIDs = append(userIDs, donation.DonorID)
			}
		} else {
			userIDs = append(userIDs, donation.AuthorID)
		}
	}

	payments, err := uc.payment.GetPayments(ctx, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("get donation payments: %w", err)
	}

	users, err := uc.identity.GetUsers(ctx, uniqueUUIDs(userIDs))
	if err != nil {
		return nil, fmt.Errorf("get donation users: %w", err)
	}

	for _, donation := range page.Items {
		payment, ok := payments[donation.PaymentOrderID]
		if !ok || payment == nil {
			return nil, fmt.Errorf(
				"payment order %s was not returned by Payments",
				donation.PaymentOrderID,
			)
		}

		userID := donation.AuthorID
		if received {
			userID = donation.DonorID
		}

		var user *dto.DonationUserInfo
		if !received || !donation.IsAnonymous {
			if info, ok := users[userID]; ok && info != nil {
				user = toDonationUserInfo(info)
			}
		}

		result.Items = append(result.Items, &dto.GetDonationResponse{
			PaymentOrderID: donation.PaymentOrderID,
			Message:        donation.Message,
			IsAnonymous:    donation.IsAnonymous,
			AmountMinor:    payment.ExpectedAmountMinor,
			Currency:       payment.Currency,
			Status:         payment.Status,
			CreatedAt:      donation.CreatedAt,
			User:           user,
		})
	}

	return result, nil
}

func toDonationUserInfo(user *identitycontracts.UserInfo) *dto.DonationUserInfo {
	if user == nil {
		return nil
	}

	var avatarURL *string
	if user.AvatarURL != "" {
		avatarURL = &user.AvatarURL
	}

	return &dto.DonationUserInfo{
		Username:  user.Username,
		Nickname:  user.Nickname,
		AvatarUrl: avatarURL,
	}
}

func uniqueUUIDs(ids []uuid.UUID) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))

	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}

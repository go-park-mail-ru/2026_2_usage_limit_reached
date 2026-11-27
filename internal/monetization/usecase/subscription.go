package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/google/uuid"
)

const (
	subscriptionTierStatusOpen     = "open"
	subscriptionTierStatusArchived = "archived"
)

func (uc *MonetizationUsecase) CreateSubscriptionTier(ctx context.Context, authorID uuid.UUID, req dto.CreateSubscriptionTierRequest) (*dto.SubscriptionTier, error) {
	req = req.Normalize()
	if err := req.Validate(uc.validator); err != nil {
		return nil, fmt.Errorf("create subscription tier: %w: %w", ErrValidation, err)
	}
	if err := uc.checkActiveAuthor(ctx, authorID, "create subscription tier"); err != nil {
		return nil, err
	}

	tier, err := uc.repo.CreateSubscriptionTier(ctx, authorID, req)
	if err != nil {
		return nil, mapRepoError("create subscription tier", err)
	}
	if tier == nil || tier.ID == uuid.Nil {
		return nil, mapRepoError("create subscription tier", errors.New("repository returned nil tier"))
	}
	if err := uc.checkActiveAuthor(ctx, authorID, "create subscription tier"); err != nil {
		return nil, err
	}

	return &dto.SubscriptionTier{
		ID:          tier.ID,
		Name:        tier.Name,
		Description: tier.Description,
		PriceMinor:  tier.PriceMinor,
		Currency:    tier.Currency,
		Status:      subscriptionTierStatusOpen,
	}, nil
}

func (uc *MonetizationUsecase) UpdateSubscriptionTier(
	ctx context.Context,
	authorID, tierID uuid.UUID,
	req dto.UpdateSubscriptionTierRequest,
) (*dto.SubscriptionTier, error) {
	req = req.Normalize()
	if err := req.Validate(uc.validator); err != nil {
		return nil, fmt.Errorf("update subscription tier: %w: %w", ErrValidation, err)
	}
	if err := uc.checkActiveAuthor(ctx, authorID, "update subscription tier"); err != nil {
		return nil, err
	}

	tier, err := uc.repo.UpdateSubscriptionTier(ctx, authorID, tierID, req)
	if err != nil {
		return nil, mapRepoError("update subscription tier", err)
	}
	if tier == nil || tier.ID == uuid.Nil {
		return nil, mapRepoError("update subscription tier", errors.New("repository returned nil tier"))
	}
	if err := uc.checkActiveAuthor(ctx, authorID, "update subscription tier"); err != nil {
		return nil, err
	}

	status := subscriptionTierStatusOpen
	if tier.ArchivedAt != nil {
		status = subscriptionTierStatusArchived
	}

	return &dto.SubscriptionTier{
		ID:          tier.ID,
		Name:        tier.Name,
		Description: tier.Description,
		PriceMinor:  tier.PriceMinor,
		Currency:    tier.Currency,
		Status:      status,
	}, nil
}

func (uc *MonetizationUsecase) GetMySubscriptionTiers(ctx context.Context, authorID uuid.UUID, cursor *dto.SubscriptionTierCursor, limit int) (*dto.SubscriptionTierPage, error) {
	tiers, err := uc.repo.GetSubscriptionTiers(ctx, authorID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get my subscription tiers", err)
	}

	page := feed.Paginate(tiers, limit, func(tier *models.SubscriptionTier) dto.SubscriptionTierCursor {
		return dto.SubscriptionTierCursor{
			PriceMinor: tier.PriceMinor,
			TierID:     tier.ID,
		}
	})

	result := &dto.SubscriptionTierPage{
		Items: make([]*dto.SubscriptionTier, 0, len(page.Items)),
		Next:  page.Next,
	}
	for _, tier := range page.Items {
		status := subscriptionTierStatusOpen
		if tier.ArchivedAt != nil {
			status = subscriptionTierStatusArchived
		}

		result.Items = append(result.Items, &dto.SubscriptionTier{
			ID:          tier.ID,
			Name:        tier.Name,
			Description: tier.Description,
			PriceMinor:  tier.PriceMinor,
			Currency:    tier.Currency,
			Status:      status,
		})
	}

	return result, nil
}

func (uc *MonetizationUsecase) GetSubscriptionOptions(
	ctx context.Context,
	readerID, authorID uuid.UUID,
	postID *uuid.UUID,
	cursor *dto.SubscriptionTierCursor,
	limit int,
) (*dto.SubscriptionOptionsPage, error) {
	if err := uc.checkActiveAuthor(ctx, authorID, "get subscription options"); err != nil {
		return nil, err
	}

	tiers, current, err := uc.repo.GetSubscriptionOptions(ctx, readerID, authorID, postID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get subscription options", err)
	}
	if err := uc.checkActiveAuthor(ctx, authorID, "get subscription options"); err != nil {
		return nil, err
	}

	page := feed.Paginate(tiers, limit, func(tier *models.SubscriptionTier) dto.SubscriptionTierCursor {
		return dto.SubscriptionTierCursor{
			PriceMinor: tier.PriceMinor,
			TierID:     tier.ID,
		}
	})
	result := &dto.SubscriptionOptionsPage{
		Items: make([]*dto.SubscriptionTier, 0, len(page.Items)),
		Next:  page.Next,
	}

	if current != nil {
		result.CurrentSubscription = &dto.CurrentSubscription{
			ID: current.ID,
			Tier: dto.SubscriptionTierReference{
				ID:     current.TierID,
				Name:   current.TierName,
				Status: current.TierStatus,
			},
			PriceMinor:   current.PriceMinor,
			Currency:     current.Currency,
			PeriodEndsAt: current.PeriodEndsAt,
		}
	}

	for _, tier := range page.Items {
		result.Items = append(result.Items, &dto.SubscriptionTier{
			ID:          tier.ID,
			Name:        tier.Name,
			Description: tier.Description,
			PriceMinor:  tier.PriceMinor,
			Currency:    tier.Currency,
		})
	}

	return result, nil
}

func (uc *MonetizationUsecase) CreateSubscription(
	ctx context.Context,
	readerID uuid.UUID,
	req dto.CreateSubscriptionRequest,
) (*dto.CurrentSubscription, error) {
	if readerID == uuid.Nil || req.AuthorID == uuid.Nil || req.TierID == uuid.Nil {
		return nil, ErrValidation
	}
	if readerID == req.AuthorID {
		return nil, ErrForbidden
	}
	if err := req.Validate(uc.validator); err != nil {
		return nil, fmt.Errorf("create subscription: %w: %w", ErrValidation, err)
	}
	if err := uc.checkActiveAuthor(ctx, req.AuthorID, "create subscription"); err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(15 * time.Minute)
	subscription, order, err := uc.repo.CreateSubscriptionWithPayment(ctx, dto.CreateSubscriptionWithPayment{
		ReaderID:               readerID,
		AuthorID:               req.AuthorID,
		Action:                 req.Action,
		TierID:                 req.TierID,
		ExpectedSubscriptionID: req.ExpectedSubscriptionID,
		ExpectedCurrentTierID:  req.ExpectedCurrentTierID,
		ExpectedPriceMinor:     req.ExpectedPriceMinor,
		PostID:                 req.PostID,
		RequestKey:             req.RequestKey,
		ExpiresAt:              &expiresAt,
	})
	if err != nil {
		return nil, mapRepoError("create subscription", err)
	}
	if subscription == nil || subscription.ID == uuid.Nil || order == nil || order.ID == uuid.Nil {
		return nil, errors.New("create subscription: repository returned incomplete purchase")
	}

	paymentStatus, err := uc.Pay(ctx, order.ID)
	if err != nil {
		return nil, fmt.Errorf("pay subscription: %w", err)
	}
	if paymentStatus != paymentStatusPaid {
		if paymentStatus == string(models.StatusFailed) || paymentStatus == string(models.StatusCanceled) {
			if err := uc.repo.FailSubscriptionPurchase(ctx, order.ID); err != nil {
				return nil, mapRepoError("fail subscription purchase", err)
			}
			return nil, mapRepoError("subscription payment failed", models.ErrSubscriptionPaymentFailed)
		}
		return nil, fmt.Errorf("subscription payment did not complete: %s", paymentStatus)
	}

	preparedSubscription := subscription
	subscription, err = uc.repo.ActivateSubscription(ctx, order.ID)
	if err != nil {
		return nil, uc.compensatePaidSubscription(
			ctx,
			preparedSubscription.ID,
			order.ID,
			"subscription_activation_failed",
			mapRepoError("activate subscription", err),
		)
	}
	if subscription == nil || subscription.ID == uuid.Nil {
		return nil, uc.compensatePaidSubscription(
			ctx,
			preparedSubscription.ID,
			order.ID,
			"subscription_activation_failed",
			errors.New("activate subscription: repository returned empty subscription"),
		)
	}

	if err := uc.checkActiveAuthor(ctx, req.AuthorID, "create subscription"); err != nil {
		return nil, uc.compensatePaidSubscription(ctx, subscription.ID, order.ID, "author_unavailable", err)
	}

	return &dto.CurrentSubscription{
		ID: subscription.ID,
		Tier: dto.SubscriptionTierReference{
			ID:     subscription.TierID,
			Name:   subscription.TierName,
			Status: subscription.TierStatus,
		},
		PriceMinor:   subscription.PriceMinor,
		Currency:     subscription.Currency,
		PeriodEndsAt: subscription.PeriodEndsAt,
	}, nil
}

func (uc *MonetizationUsecase) compensatePaidSubscription(
	ctx context.Context,
	subscriptionID, orderID uuid.UUID,
	reason string,
	cause error,
) error {
	cancelErr := uc.repo.CancelSubscription(ctx, subscriptionID)
	refundErr := uc.paymentRepo.RefundPaidOrder(ctx, orderID, reason)
	return fmt.Errorf("compensate subscription purchase: %w", errors.Join(cause, cancelErr, refundErr))
}

func (uc *MonetizationUsecase) checkActiveAuthor(ctx context.Context, authorID uuid.UUID, operation string) error {
	if authorID == uuid.Nil {
		return ErrValidation
	}

	users, err := uc.identity.GetUsers(ctx, []uuid.UUID{authorID})
	if err != nil {
		return fmt.Errorf("%s author lookup: %w", operation, err)
	}
	if author, ok := users[authorID]; !ok || author == nil || author.Status != "active" {
		return ErrNotFound
	}

	return nil
}

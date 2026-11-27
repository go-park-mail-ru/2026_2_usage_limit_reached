package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	paymentProvider "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider"
	"github.com/google/uuid"
)

type PaymentRepository interface {
	GetOrdersByIDs(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]*models.PaymentOrder, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (*models.PaymentOrder, error)
	StartAttempt(ctx context.Context, orderID uuid.UUID) (*models.PaymentOrder, *models.PaymentAttempt, bool, error)
	CompleteSuccessfulAttempt(ctx context.Context, attempt dto.AttemptToComplete) (bool, error)
	FailAttempt(ctx context.Context, orderID, attemptID uuid.UUID, reason string) error
	RefundPaidOrder(ctx context.Context, orderID uuid.UUID, reason string) error
}

func (uc *MonetizationUsecase) GetPayments(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]*models.PaymentOrder, error) {
	if len(orderIDs) == 0 {
		return map[uuid.UUID]*models.PaymentOrder{}, nil
	}

	payments, err := uc.paymentRepo.GetOrdersByIDs(ctx, orderIDs)
	if err != nil {
		return nil, mapRepoError("get payments", err)
	}

	for id, order := range payments {
		if order == nil {
			return nil, fmt.Errorf("get payments: repository returned an empty order for %s", id)
		}
	}

	return payments, nil
}

func (uc *MonetizationUsecase) Pay(ctx context.Context, orderID uuid.UUID) (string, error) {
	if orderID == uuid.Nil {
		return "", ErrValidation
	}

	order, attempt, started, err := uc.paymentRepo.StartAttempt(ctx, orderID)
	if err != nil {
		return "", mapRepoError("start payment attempt", err)
	}
	if order == nil || attempt == nil {
		return "", errors.New("start payment attempt: repository returned empty payment state")
	}

	if attempt.PaymentOrderID != orderID {
		return "", ErrValidation
	}

	// Заказ уже обрабатывается или находится в конечном состоянии.
	if !started {
		return string(order.Status), nil
	}

	res, err := uc.paymentProvider.Charge(ctx, paymentProvider.ChargeRequest{
		AttemptID:   attempt.ID,
		AmountMinor: order.ExpectedAmountMinor,
		Currency:    order.Currency,
	})
	if err != nil {
		return string(order.Status), fmt.Errorf("charge: %w", err)
	}

	switch res.Status {
	case paymentProvider.ChargeSucceeded:
		if _, err := uc.paymentRepo.CompleteSuccessfulAttempt(ctx, dto.AttemptToComplete{
			ID:                    attempt.ID,
			OrderID:               orderID,
			ExternalTransactionID: res.ExternalID,
			ChargedAmountMinor:    res.AmountMinor,
			ChargedCurrency:       res.Currency,
			CompletedAt:           time.Now(),
		}); err != nil {
			return "", mapRepoError("complete attempt", err)
		}
	case paymentProvider.ChargeFailed:
		if err := uc.paymentRepo.FailAttempt(ctx, orderID, attempt.ID, res.FailureReason); err != nil {
			return "", mapRepoError("fail attempt", err)
		}
	}

	final, err := uc.paymentRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return "", mapRepoError("get final order", err)
	}

	return string(final.Status), nil
}

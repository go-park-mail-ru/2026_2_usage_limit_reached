package paymentsusecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/dto"
	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	paymentProvider "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider"
	"github.com/google/uuid"
)

var (
	ErrForbidden            = errors.New("forbidden")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrValidation           = errors.New("validation error")
	ErrNotFound             = errors.New("not found")
	ErrIdempotencyKeyReused = errors.New("request key conflict")
)

type PaymentRepo interface {
	CreateOrGetOrderWithAttempt(ctx context.Context, req dto.CreatePayment) (orderID, attemptID uuid.UUID, err error)
	GetOrdersByIDs(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]*models.PaymentOrder, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (*models.PaymentOrder, error)
	CompleteSuccessfulAttempt(ctx context.Context, attempt dto.AttemptToComplete) (bool, error)
	FailAttempt(ctx context.Context, orderID, attemptID uuid.UUID, reason string) error
	GetAttemptByID(ctx context.Context, id uuid.UUID) (*models.PaymentAttempt, error)
}

var repoErrors = []struct{ from, to error }{
	{models.ErrPaymentOrderNotFound, ErrNotFound},
	{models.ErrPaymentAttemptNotFound, ErrNotFound},
	{models.ErrForbiddenUpdate, ErrForbidden},
	{models.ErrPaymentOrderValidation, ErrValidation},
	{models.ErrPaymentAttemptValidation, ErrValidation},
	{models.ErrDuplicateRequestKey, ErrIdempotencyKeyReused},
}

func mapRepoError(op string, err error) error {
	for _, e := range repoErrors {
		if errors.Is(err, e.from) {
			return fmt.Errorf("%s: %w", op, e.to)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

type PaymentUsecase struct {
	repo            PaymentRepo
	paymentProvider paymentProvider.Provider
}

func NewPaymentUsecase(repo PaymentRepo, provider paymentProvider.Provider) *PaymentUsecase {
	return &PaymentUsecase{repo: repo, paymentProvider: provider}
}

func (uc *PaymentUsecase) CreatePayment(ctx context.Context, req dto.CreatePayment) (*models.PaymentOrder, error) {
	orderID, attemptID, err := uc.repo.CreateOrGetOrderWithAttempt(ctx, req)
	if err != nil {
		return nil, mapRepoError("create payment", err)
	}

	if _, err := uc.Pay(ctx, orderID, attemptID); err != nil {
		return nil, fmt.Errorf("pay: %w", err)
	}

	order, err := uc.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, mapRepoError("get created payment", err)
	}

	return order, nil
}

func (uc *PaymentUsecase) GetPayments(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]*models.PaymentOrder, error) {
	if len(orderIDs) == 0 {
		return map[uuid.UUID]*models.PaymentOrder{}, nil
	}

	payments, err := uc.repo.GetOrdersByIDs(ctx, orderIDs)
	if err != nil {
		return nil, mapRepoError("get payments", err)
	}

	return payments, nil
}

func (uc *PaymentUsecase) Pay(ctx context.Context, orderID, attemptID uuid.UUID) (models.PaymentOrderStatus, error) {
	order, err := uc.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return "", mapRepoError("get order", err)
	}

	attempt, err := uc.repo.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return "", mapRepoError("get attempt", err)
	}

	if attempt.PaymentOrderID != orderID {
		return "", ErrValidation
	}

	if order.Status == models.StatusPaid ||
		order.Status == models.StatusFailed ||
		order.Status == models.StatusCanceled ||
		attempt.Status != models.AttemptStatusCreated {
		return order.Status, nil
	}

	res, err := uc.paymentProvider.Charge(ctx, paymentProvider.ChargeRequest{
		AttemptID:   attemptID,
		AmountMinor: order.ExpectedAmountMinor,
		Currency:    order.Currency,
	})
	if err != nil {
		return order.Status, fmt.Errorf("charge: %w", err)
	}

	switch res.Status {
	case paymentProvider.ChargeSucceeded:
		if _, err := uc.repo.CompleteSuccessfulAttempt(ctx, dto.AttemptToComplete{
			ID:                    attemptID,
			OrderID:               orderID,
			ExternalTransactionID: res.ExternalID,
			ChargedAmountMinor:    res.AmountMinor,
			ChargedCurrency:       res.Currency,
			CompletedAt:           time.Now(),
		}); err != nil {
			return "", mapRepoError("complete attempt", err)
		}
	case paymentProvider.ChargeFailed:
		if err := uc.repo.FailAttempt(ctx, orderID, attemptID, res.FailureReason); err != nil {
			return "", mapRepoError("fail attempt", err)
		}
	}

	final, err := uc.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return "", mapRepoError("get order", err)
	}
	return final.Status, nil
}

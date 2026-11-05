package paymentsusecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/dto"
	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	providermock "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/provider/mock"
	"github.com/google/uuid"
)

var (
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
	ErrValidation   = errors.New("validation error")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("post update conflict")
)

type PaymentOrderRepo interface {
	GetOrderByID(ctx context.Context, id uuid.UUID) (*models.PaymentOrder, error)
	CompleteSuccessfulAttempt(ctx context.Context, attempt dto.AttemptToComplete) (bool, error)
	FailAttempt(ctx context.Context, orderID, attemptID uuid.UUID, reason string) error
	Cancel(ctx context.Context, userID, orderID uuid.UUID, failureReason string) error
}

type PaymentAttemptRepo interface {
	Create(ctx context.Context, attempt *models.PaymentAttempt) (*models.PaymentAttempt, error)
	GetAttemptByID(ctx context.Context, id uuid.UUID) (*models.PaymentAttempt, error)
}

var repoErrors = map[error]error{
	models.ErrPaymentOrderNotFound:     ErrNotFound,
	models.ErrPaymentAttemptNotFound:   ErrNotFound,
	models.ErrForbiddenUpdate:          ErrForbidden,
	models.ErrPaymentOrderValidation:   ErrValidation,
	models.ErrPaymentAttemptValidation: ErrValidation,
	models.ErrDuplicateRequestKey:      ErrConflict,
}

func mapRepoError(op string, err error) error {
	to, ok := repoErrors[err]
	if ok {
		return fmt.Errorf("%s: %w", op, to)
	}
	return fmt.Errorf("%s: %w", op, err)
}

type PaymentUsecase struct {
	orders   PaymentOrderRepo
	attempts PaymentAttemptRepo
	provider providermock.Provider
}

func (uc *PaymentUsecase) Pay(ctx context.Context, orderID, attemptID uuid.UUID) (models.PaymentOrderStatus, error) {
	order, err := uc.orders.GetOrderByID(ctx, orderID)
	if err != nil {
		return "", mapRepoError("get order", err)
	}

	attempt, err := uc.attempts.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return "", mapRepoError("get attempt", err)
	}

	if attempt.PaymentOrderID != orderID {
		return "", ErrValidation
	}

	switch order.Status {
	case models.StatusPaid, models.StatusFailed, models.StatusCanceled:
		return order.Status, nil
	}

	res, err := uc.provider.Charge(ctx, providermock.ChargeRequest{
		AttemptID:   attemptID,
		AmountMinor: order.ExpectedAmountMinor,
		Currency:    order.Currency,
	})
	if err != nil {
		return order.Status, fmt.Errorf("charge: %w", err)
	}

	switch res.Status {
	case providermock.ChargeSucceeded:
		if _, err := uc.orders.CompleteSuccessfulAttempt(ctx, dto.AttemptToComplete{
			ID:                    attemptID,
			OrderID:               orderID,
			ExternalTransactionID: res.ExternalID,
			ChargedAmountMinor:    res.AmountMinor,
			ChargedCurrency:       res.Currency,
			CompletedAt:           time.Now(),
		}); err != nil {
			return "", mapRepoError("complete attempt", err)
		}
	case providermock.ChargeFailed:
		if err := uc.orders.FailAttempt(ctx, orderID, attemptID, res.FailureReason); err != nil {
			return "", mapRepoError("fail attempt", err)
		}
	}

	final, err := uc.orders.GetOrderByID(ctx, orderID)
	if err != nil {
		return "", mapRepoError("get order", err)
	}
	return final.Status, nil
}

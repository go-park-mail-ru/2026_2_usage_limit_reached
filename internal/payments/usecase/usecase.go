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
	CreateOrGetOrder(ctx context.Context, req dto.CreatePayment) (orderID uuid.UUID, err error)
	GetOrdersByIDs(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]*models.PaymentOrder, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (*models.PaymentOrder, error)
	StartAttempt(ctx context.Context, orderID uuid.UUID) (*models.PaymentOrder, *models.PaymentAttempt, bool, error)
	CompleteSuccessfulAttempt(ctx context.Context, attempt dto.AttemptToComplete) (bool, error)
	FailAttempt(ctx context.Context, orderID, attemptID uuid.UUID, reason string) error
	CancelOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentOrder, error)
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
	orderID, err := uc.repo.CreateOrGetOrder(ctx, req)
	if err != nil {
		return nil, mapRepoError("create payment", err)
	}

	order, err := uc.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, mapRepoError("get created payment order", err)
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

func (uc *PaymentUsecase) CancelPayment(ctx context.Context, orderID uuid.UUID) (*models.PaymentOrder, error) {
	if orderID == uuid.Nil {
		return nil, ErrValidation
	}

	order, err := uc.repo.CancelOrder(ctx, orderID)
	if err != nil {
		return nil, mapRepoError("cancel payment", err)
	}

	return order, nil
}

func (uc *PaymentUsecase) Pay(ctx context.Context, orderID uuid.UUID) (models.PaymentOrderStatus, error) {
	if orderID == uuid.Nil {
		return "", ErrValidation
	}

	order, attempt, started, err := uc.repo.StartAttempt(ctx, orderID)
	if err != nil {
		return "", mapRepoError("start payment attempt", err)
	}

	if attempt.PaymentOrderID != orderID {
		return "", ErrValidation
	}

	// Заказ уже обрабатывается или находится в конечном состоянии.
	if !started {
		return order.Status, nil
	}

	res, err := uc.paymentProvider.Charge(ctx, paymentProvider.ChargeRequest{
		AttemptID:   attempt.ID,
		AmountMinor: order.ExpectedAmountMinor,
		Currency:    order.Currency,
	})
	if err != nil {
		return order.Status, fmt.Errorf("charge: %w", err)
	}

	switch res.Status {
	case paymentProvider.ChargeSucceeded:
		if _, err := uc.repo.CompleteSuccessfulAttempt(ctx, dto.AttemptToComplete{
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
		if err := uc.repo.FailAttempt(ctx, orderID, attempt.ID, res.FailureReason); err != nil {
			return "", mapRepoError("fail attempt", err)
		}
	}

	final, err := uc.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return "", mapRepoError("get final order", err)
	}

	return final.Status, nil
}

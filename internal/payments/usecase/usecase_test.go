package paymentsusecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/dto"
	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	paymentsusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/usecase/mock_payments_usecase"
	paymentprovider "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider/mock_pkg_payment_provider"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	orderID   = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	attemptID = uuid.MustParse("00000000-0000-4000-8000-000000000002")
)

const (
	amountMinor = int64(15000)
	currency    = "RUB"
)

func order(status models.PaymentOrderStatus) *models.PaymentOrder {
	return &models.PaymentOrder{
		ID:                  orderID,
		Status:              status,
		ExpectedAmountMinor: amountMinor,
		Currency:            currency,
	}
}

func attempt(orderID uuid.UUID) *models.PaymentAttempt {
	return attemptWithStatus(orderID, models.AttemptStatusCreated)
}

func attemptWithStatus(orderID uuid.UUID, status models.PaymentAttemptStatus) *models.PaymentAttempt {
	return &models.PaymentAttempt{
		ID:             attemptID,
		PaymentOrderID: orderID,
		Status:         status,
	}
}

func chargeRequest() paymentprovider.ChargeRequest {
	return paymentprovider.ChargeRequest{
		AttemptID:   attemptID,
		AmountMinor: amountMinor,
		Currency:    currency,
	}
}

func TestPaymentUsecase_Pay(t *testing.T) {
	repoErr := errors.New("db down")
	chargeErr := errors.New("provider unavailable")

	tests := []struct {
		name           string
		mockBehavior   func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider)
		expectedStatus models.PaymentOrderStatus
		expectedErr    error
	}{
		{
			name: "StartAttempt repository error is mapped",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(nil, nil, false, repoErr)
			},
			expectedErr: repoErr,
		},
		{
			name: "Order not found",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(nil, nil, false, models.ErrPaymentOrderNotFound)
			},
			expectedErr: paymentsusecase.ErrNotFound,
		},
		{
			name: "Attempt not found",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(nil, nil, false, models.ErrPaymentAttemptNotFound)
			},
			expectedErr: paymentsusecase.ErrNotFound,
		},
		{
			name: "Paid order is returned without charge",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusPaid), attemptWithStatus(orderID, models.AttemptStatusSucceeded), false, nil)
			},
			expectedStatus: models.StatusPaid,
		},
		{
			name: "Failed order is returned without charge",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusFailed), attemptWithStatus(orderID, models.AttemptStatusFailed), false, nil)
			},
			expectedStatus: models.StatusFailed,
		},
		{
			name: "Canceled order is returned without charge",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusCanceled), attemptWithStatus(orderID, models.AttemptStatusCanceled), false, nil)
			},
			expectedStatus: models.StatusCanceled,
		},
		{
			name: "Already processing attempt is not charged again",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusProcessing), attemptWithStatus(orderID, models.AttemptStatusProcessing), false, nil)
			},
			expectedStatus: models.StatusProcessing,
		},
		{
			name: "Expired pending order is not charged",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				// StartAttempt возвращает started=false, если expires_at уже наступил
				// Сам заказ отменит expiration worker
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusPending), attempt(orderID), false, nil)
			},
			expectedStatus: models.StatusPending,
		},
		{
			name: "Charge error leaves order processing",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusProcessing), attemptWithStatus(orderID, models.AttemptStatusProcessing), true, nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).
					Return(paymentprovider.ChargeResult{}, chargeErr)
			},
			expectedStatus: models.StatusProcessing,
			expectedErr:    chargeErr,
		},
		{
			name: "Successful charge completes attempt and returns final order status",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusProcessing), attemptWithStatus(orderID, models.AttemptStatusProcessing), true, nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{
					Status:      paymentprovider.ChargeSucceeded,
					ExternalID:  "ext-1",
					AmountMinor: amountMinor,
					Currency:    currency,
				}, nil)
				repo.EXPECT().CompleteSuccessfulAttempt(mock.Anything, mock.MatchedBy(func(a dto.AttemptToComplete) bool {
					return a.ID == attemptID && a.OrderID == orderID &&
						a.ExternalTransactionID == "ext-1" &&
						a.ChargedAmountMinor == amountMinor && a.ChargedCurrency == currency &&
						!a.CompletedAt.IsZero()
				})).Return(false, nil)
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).
					Return(order(models.StatusPaid), nil).Once()
			},
			expectedStatus: models.StatusPaid,
		},
		{
			name: "Successful late charge returns the current canceled order state",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusProcessing), attemptWithStatus(orderID, models.AttemptStatusProcessing), true, nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{
					Status:      paymentprovider.ChargeSucceeded,
					ExternalID:  "ext-late",
					AmountMinor: amountMinor,
					Currency:    currency,
				}, nil)
				repo.EXPECT().CompleteSuccessfulAttempt(mock.Anything, mock.Anything).Return(true, nil)
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).
					Return(order(models.StatusCanceled), nil).Once()
			},
			expectedStatus: models.StatusCanceled,
		},
		{
			name: "Complete attempt error is mapped",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusProcessing), attemptWithStatus(orderID, models.AttemptStatusProcessing), true, nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{
					Status: paymentprovider.ChargeSucceeded,
				}, nil)
				repo.EXPECT().CompleteSuccessfulAttempt(mock.Anything, mock.Anything).
					Return(false, models.ErrForbiddenUpdate)
			},
			expectedErr: paymentsusecase.ErrForbidden,
		},
		{
			name: "Failed charge fails attempt and returns final order status",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusProcessing), attemptWithStatus(orderID, models.AttemptStatusProcessing), true, nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{
					Status:        paymentprovider.ChargeFailed,
					FailureReason: "insufficient funds",
				}, nil)
				repo.EXPECT().FailAttempt(mock.Anything, orderID, attemptID, "insufficient funds").Return(nil)
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).
					Return(order(models.StatusFailed), nil).Once()
			},
			expectedStatus: models.StatusFailed,
		},
		{
			name: "Fail attempt error is mapped",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusProcessing), attemptWithStatus(orderID, models.AttemptStatusProcessing), true, nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{
					Status: paymentprovider.ChargeFailed,
				}, nil)
				repo.EXPECT().FailAttempt(mock.Anything, orderID, attemptID, "").
					Return(models.ErrPaymentAttemptValidation)
			},
			expectedErr: paymentsusecase.ErrValidation,
		},
		{
			name: "Unknown charge status reloads current order",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusProcessing), attemptWithStatus(orderID, models.AttemptStatusProcessing), true, nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).
					Return(paymentprovider.ChargeResult{Status: "pending"}, nil)
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).
					Return(order(models.StatusProcessing), nil).Once()
			},
			expectedStatus: models.StatusProcessing,
		},
		{
			name: "Final order reload error is mapped",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().StartAttempt(mock.Anything, orderID).
					Return(order(models.StatusProcessing), attemptWithStatus(orderID, models.AttemptStatusProcessing), true, nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{
					Status: paymentprovider.ChargeSucceeded,
				}, nil)
				repo.EXPECT().CompleteSuccessfulAttempt(mock.Anything, mock.Anything).Return(false, nil)
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).
					Return(nil, models.ErrPaymentOrderNotFound).Once()
			},
			expectedErr: paymentsusecase.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mock_payments_usecase.NewMockPaymentRepo(t)
			provider := mock_pkg_payment_provider.NewMockProvider(t)
			tt.mockBehavior(repo, provider)

			uc := paymentsusecase.NewPaymentUsecase(repo, provider)
			status, err := uc.Pay(context.Background(), orderID)

			require.Equal(t, tt.expectedStatus, status)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPaymentUsecase_Pay_RepoErrorMapping(t *testing.T) {
	unmapped := errors.New("db down")

	tests := []struct {
		name     string
		repoErr  error
		expected error
	}{
		{name: "Order not found", repoErr: models.ErrPaymentOrderNotFound, expected: paymentsusecase.ErrNotFound},
		{name: "Attempt not found", repoErr: models.ErrPaymentAttemptNotFound, expected: paymentsusecase.ErrNotFound},
		{name: "Forbidden update", repoErr: models.ErrForbiddenUpdate, expected: paymentsusecase.ErrForbidden},
		{name: "Order validation", repoErr: models.ErrPaymentOrderValidation, expected: paymentsusecase.ErrValidation},
		{name: "Attempt validation", repoErr: models.ErrPaymentAttemptValidation, expected: paymentsusecase.ErrValidation},
		{name: "Duplicate request key", repoErr: models.ErrDuplicateRequestKey, expected: paymentsusecase.ErrIdempotencyKeyReused},
		{name: "Unmapped error passes through", repoErr: unmapped, expected: unmapped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mock_payments_usecase.NewMockPaymentRepo(t)
			provider := mock_pkg_payment_provider.NewMockProvider(t)
			repo.EXPECT().StartAttempt(mock.Anything, orderID).
				Return(nil, nil, false, tt.repoErr)

			uc := paymentsusecase.NewPaymentUsecase(repo, provider)
			status, err := uc.Pay(context.Background(), orderID)

			require.ErrorIs(t, err, tt.expected)
			require.Empty(t, status)
		})
	}
}

func TestPaymentUsecase_CancelPayment(t *testing.T) {
	dbErr := errors.New("db down")
	canceledOrder := order(models.StatusCanceled)

	tests := []struct {
		name         string
		orderID      uuid.UUID
		mockBehavior func(repo *mock_payments_usecase.MockPaymentRepo)
		wantOrder    *models.PaymentOrder
		wantErr      error
	}{
		{
			name:    "Cancel returns order from repository",
			orderID: orderID,
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo) {
				repo.EXPECT().CancelOrder(mock.Anything, orderID).Return(canceledOrder, nil).Once()
			},
			wantOrder: canceledOrder,
		},
		{
			name:    "Already canceled order is returned idempotently",
			orderID: orderID,
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo) {
				// Идемпотентность реализована в репозитории: он возвращает текущую запись
				repo.EXPECT().CancelOrder(mock.Anything, orderID).Return(canceledOrder, nil).Once()
			},
			wantOrder: canceledOrder,
		},
		{
			name:    "Order not found",
			orderID: orderID,
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo) {
				repo.EXPECT().CancelOrder(mock.Anything, orderID).
					Return(nil, models.ErrPaymentOrderNotFound)
			},
			wantErr: paymentsusecase.ErrNotFound,
		},
		{
			name:    "Paid or otherwise non-cancelable order is forbidden",
			orderID: orderID,
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo) {
				repo.EXPECT().CancelOrder(mock.Anything, orderID).
					Return(nil, models.ErrForbiddenUpdate)
			},
			wantErr: paymentsusecase.ErrForbidden,
		},
		{
			name:    "Repository validation error is mapped",
			orderID: orderID,
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo) {
				repo.EXPECT().CancelOrder(mock.Anything, orderID).
					Return(nil, models.ErrPaymentOrderValidation)
			},
			wantErr: paymentsusecase.ErrValidation,
		},
		{
			name:    "Unmapped repository error passes through",
			orderID: orderID,
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo) {
				repo.EXPECT().CancelOrder(mock.Anything, orderID).Return(nil, dbErr)
			},
			wantErr: dbErr,
		},
		{
			name:    "Nil order ID is rejected before repository call",
			orderID: uuid.Nil,
			mockBehavior: func(_ *mock_payments_usecase.MockPaymentRepo) {
				// Вызова репозитория быть не должно
			},
			wantErr: paymentsusecase.ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mock_payments_usecase.NewMockPaymentRepo(t)
			provider := mock_pkg_payment_provider.NewMockProvider(t)
			tt.mockBehavior(repo)

			uc := paymentsusecase.NewPaymentUsecase(repo, provider)
			gotOrder, err := uc.CancelPayment(context.Background(), tt.orderID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, gotOrder)
				return
			}

			require.NoError(t, err)
			require.Same(t, tt.wantOrder, gotOrder)
		})
	}
}

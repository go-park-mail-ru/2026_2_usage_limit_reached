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
	otherID   = uuid.MustParse("00000000-0000-4000-8000-000000000003")
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
	return &models.PaymentAttempt{ID: attemptID, PaymentOrderID: orderID}
}

func chargeRequest() paymentprovider.ChargeRequest {
	return paymentprovider.ChargeRequest{AttemptID: attemptID, AmountMinor: amountMinor, Currency: currency}
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
			name: "Order repository error is not mapped",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(nil, repoErr)
			},
			expectedErr: repoErr,
		},
		{
			name: "Order not found",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(nil, models.ErrPaymentOrderNotFound)
			},
			expectedErr: paymentsusecase.ErrNotFound,
		},
		{
			name: "Attempt not found",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPending), nil)
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(nil, models.ErrPaymentAttemptNotFound)
			},
			expectedErr: paymentsusecase.ErrNotFound,
		},
		{
			name: "Attempt belongs to another order",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPending), nil)
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(otherID), nil)
			},
			expectedErr: paymentsusecase.ErrValidation,
		},
		{
			name: "Paid order is returned without charge",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPaid), nil)
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(orderID), nil)
			},
			expectedStatus: models.StatusPaid,
		},
		{
			name: "Failed order is returned without charge",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusFailed), nil)
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(orderID), nil)
			},
			expectedStatus: models.StatusFailed,
		},
		{
			name: "Canceled order is returned without charge",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, _ *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusCanceled), nil)
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(orderID), nil)
			},
			expectedStatus: models.StatusCanceled,
		},
		{
			name: "Charge error keeps current status",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPending), nil)
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(orderID), nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{}, chargeErr)
			},
			expectedStatus: models.StatusPending,
			expectedErr:    chargeErr,
		},
		{
			name: "Successful charge completes attempt and returns final status",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPending), nil).Once()
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(orderID), nil)
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
				})).Return(true, nil)
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPaid), nil).Once()
			},
			expectedStatus: models.StatusPaid,
		},
		{
			name: "Complete attempt error is mapped",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPending), nil)
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(orderID), nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{Status: paymentprovider.ChargeSucceeded}, nil)
				repo.EXPECT().CompleteSuccessfulAttempt(mock.Anything, mock.Anything).Return(false, models.ErrForbiddenUpdate)
			},
			expectedErr: paymentsusecase.ErrForbidden,
		},
		{
			name: "Failed charge fails attempt and returns final status",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPending), nil).Once()
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(orderID), nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{
					Status:        paymentprovider.ChargeFailed,
					FailureReason: "insufficient funds",
				}, nil)
				repo.EXPECT().FailAttempt(mock.Anything, orderID, attemptID, "insufficient funds").Return(nil)
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusFailed), nil).Once()
			},
			expectedStatus: models.StatusFailed,
		},
		{
			name: "Fail attempt error is mapped",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPending), nil)
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(orderID), nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{Status: paymentprovider.ChargeFailed}, nil)
				repo.EXPECT().FailAttempt(mock.Anything, orderID, attemptID, "").Return(models.ErrPaymentAttemptValidation)
			},
			expectedErr: paymentsusecase.ErrValidation,
		},
		{
			name: "Unknown charge status only reloads order",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPending), nil).Once()
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(orderID), nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{Status: "pending"}, nil)
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusProcessing), nil).Once()
			},
			expectedStatus: models.StatusProcessing,
		},
		{
			name: "Final order reload error is mapped",
			mockBehavior: func(repo *mock_payments_usecase.MockPaymentRepo, provider *mock_pkg_payment_provider.MockProvider) {
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(order(models.StatusPending), nil).Once()
				repo.EXPECT().GetAttemptByID(mock.Anything, attemptID).Return(attempt(orderID), nil)
				provider.EXPECT().Charge(mock.Anything, chargeRequest()).Return(paymentprovider.ChargeResult{Status: paymentprovider.ChargeSucceeded}, nil)
				repo.EXPECT().CompleteSuccessfulAttempt(mock.Anything, mock.Anything).Return(true, nil)
				repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(nil, models.ErrPaymentOrderNotFound).Once()
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

			status, err := uc.Pay(context.Background(), orderID, attemptID)

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
			repo.EXPECT().GetOrderByID(mock.Anything, orderID).Return(nil, tt.repoErr)
			uc := paymentsusecase.NewPaymentUsecase(repo, provider)

			status, err := uc.Pay(context.Background(), orderID, attemptID)

			require.ErrorIs(t, err, tt.expected)
			require.Empty(t, status)
		})
	}
}

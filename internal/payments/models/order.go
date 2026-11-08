package paymentmodels

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPaymentOrderNotFound   = errors.New("payment order not found")
	ErrPaymentOrderValidation = errors.New("payment order validation error")
	ErrForbiddenUpdate        = errors.New("payment order update forbidden")
	ErrDuplicateRequestKey    = errors.New("duplicate request key")
)

type PaymentOrderPurpose string

const (
	PurposeDonation PaymentOrderPurpose = "donation"
)

type PaymentOrderOrigin string

const (
	OriginManual      PaymentOrderOrigin = "manual"
	OriginAutoRenewal PaymentOrderOrigin = "auto_renewal"
)

type PaymentOrderStatus string

const (
	StatusPending    PaymentOrderStatus = "pending"
	StatusProcessing PaymentOrderStatus = "processing"
	StatusPaid       PaymentOrderStatus = "paid"
	StatusFailed     PaymentOrderStatus = "failed"
	StatusCanceled   PaymentOrderStatus = "canceled"
)

type PaymentOrder struct {
	ID                  uuid.UUID
	PayerID             uuid.UUID
	RequestKey          string
	Purpose             PaymentOrderPurpose
	Origin              PaymentOrderOrigin
	ExpectedAmountMinor int64
	Currency            string
	Status              PaymentOrderStatus
	CreditedAttemptID   *uuid.UUID
	ExpiresAt           *time.Time
	PaidAt              *time.Time
	CanceledAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type OrderState struct {
	Status            PaymentOrderStatus
	ExpectedAmount    int64
	ExpectedCurrency  string
	CreditedAttemptID *uuid.UUID
}

type AttemptState struct {
	OrderID               uuid.UUID
	Status                PaymentAttemptStatus
	ExternalTransactionID *string
	ChargedAmount         *int64
	ChargedCurrency       *string
}

type SuccessfulPayment struct {
	AttemptID             uuid.UUID
	OrderID               uuid.UUID
	ExternalTransactionID string
	ChargedAmount         int64
	ChargedCurrency       string
}

type RefundReason string

const (
	RefundDuplicate      RefundReason = "duplicate_payment"
	RefundOrderCanceled  RefundReason = "order_canceled"
	RefundAmountMismatch RefundReason = "amount_mismatch"
)

type OrderDecision struct {
	SaveAttempt bool
	FailOrder   bool
	MarkPaid    bool
	Refund      RefundReason
}

func (d OrderDecision) RequiresRefund() bool { return d.Refund != "" }

func (a AttemptState) matchesPayment(p SuccessfulPayment) bool {
	return a.OrderID == p.OrderID &&
		a.ExternalTransactionID != nil && *a.ExternalTransactionID == p.ExternalTransactionID &&
		a.ChargedAmount != nil && *a.ChargedAmount == p.ChargedAmount &&
		a.ChargedCurrency != nil && *a.ChargedCurrency == p.ChargedCurrency
}

func MakeOrderDecision(order OrderState, attempt AttemptState, payment SuccessfulPayment) (OrderDecision, error) {
	var d OrderDecision

	if attempt.Status == AttemptStatusSucceeded {
		if !attempt.matchesPayment(payment) {
			return d, ErrPaymentAttemptValidation
		}
	} else {
		d.SaveAttempt = true
	}

	switch order.Status {
	case StatusPaid:
		if order.CreditedAttemptID == nil {
			return d, ErrPaymentOrderValidation
		}
		if *order.CreditedAttemptID != payment.AttemptID {
			d.Refund = RefundDuplicate
		}
		return d, nil
	case StatusCanceled:
		d.Refund = RefundOrderCanceled
		return d, nil
	}

	if order.ExpectedAmount != payment.ChargedAmount || order.ExpectedCurrency != payment.ChargedCurrency {
		d.FailOrder = order.Status == StatusPending || order.Status == StatusProcessing
		d.Refund = RefundAmountMismatch
		return d, nil
	}

	switch order.Status {
	case StatusPending, StatusProcessing, StatusFailed:
		d.MarkPaid = true
		return d, nil
	}

	return d, ErrPaymentOrderValidation
}

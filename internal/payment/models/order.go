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

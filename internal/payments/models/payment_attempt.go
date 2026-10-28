package paymentmodels

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPaymentAttemptNotFound   = errors.New("payment attempt not found")
	ErrPaymentAttemptValidation = errors.New("payment attempt validation error")
)

type PaymentAttemptStatus string

const (
	AttemptStatusCreated    PaymentAttemptStatus = "created"
	AttemptStatusProcessing PaymentAttemptStatus = "processing"
	AttemptStatusSucceeded  PaymentAttemptStatus = "succeeded"
	AttemptStatusFailed     PaymentAttemptStatus = "failed"
	AttemptStatusCanceled   PaymentAttemptStatus = "canceled"
)

type PaymentAttempt struct {
	ID                    uuid.UUID
	PaymentOrderID        uuid.UUID
	Provider              string
	ExternalTransactionID *string
	Status                PaymentAttemptStatus
	ChargedAmountMinor    *int64
	ChargedCurrency       *string
	FailureReason         *string
	CompletedAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

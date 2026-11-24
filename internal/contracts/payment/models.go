package payment

import (
	"time"

	"github.com/google/uuid"
)

type CreatePaymentRequest struct {
	PayerID     uuid.UUID
	RequestKey  string
	Purpose     string
	Origin      string
	AmountMinor int64
	Currency    string
	ExpiresAt   *time.Time
}

type PaymentOrder struct {
	ID                  uuid.UUID
	PayerID             uuid.UUID
	Purpose             string
	Origin              string
	ExpectedAmountMinor int64
	Currency            string
	Status              string
	CreditedAttemptID   *uuid.UUID
	PaidAt              *time.Time
	CanceledAt          *time.Time
}

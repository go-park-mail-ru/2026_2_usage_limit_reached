package dto

import (
	"time"

	"github.com/google/uuid"
)

type AttemptToComplete struct {
	ID                    uuid.UUID
	OrderID               uuid.UUID
	ExternalTransactionID string
	ChargedAmountMinor    int64
	ChargedCurrency       string
	CompletedAt           time.Time
}

type CreateDonationWithPayment struct {
	DonorID     uuid.UUID
	AuthorID    uuid.UUID
	RequestKey  string
	Message     *string
	IsAnonymous bool
	AmountMinor int64
	Currency    string
	ExpiresAt   *time.Time
}

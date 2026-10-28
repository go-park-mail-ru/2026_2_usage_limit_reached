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

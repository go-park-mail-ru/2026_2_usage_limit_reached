package provider

import (
	"context"

	"github.com/google/uuid"
)

type ChargeStatus string

const (
	ChargeSucceeded ChargeStatus = "succeeded"
	ChargeFailed    ChargeStatus = "failed"
)

type ChargeRequest struct {
	AttemptID   uuid.UUID
	AmountMinor int64
	Currency    string
}

type ChargeResult struct {
	Status        ChargeStatus
	ExternalID    string
	AmountMinor   int64
	Currency      string
	FailureReason string
}

type Provider interface {
	Name() string
	Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}

package models

import (
	"time"

	"github.com/google/uuid"
)

type SubscriptionTier struct {
	ID          uuid.UUID
	AuthorID    uuid.UUID
	Name        string
	Description string
	PriceMinor  int64
	Currency    string
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Subscription struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	BillingPlanID      uuid.UUID
	Status             string
	StartedAt          time.Time
	PeriodStartedAt    time.Time
	PeriodEndsAt       time.Time
	SubscriptionTypeID uuid.UUID
}

type SubscriptionPeriod struct {
	PaymentOrderID uuid.UUID
	SubscriptionID uuid.UUID
	BillingPlanID  uuid.UUID
	PeriodStart    time.Time
	PeriodEnd      time.Time
	RevokedAt      *time.Time
}

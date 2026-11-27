package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSubscriptionTierConflict   = errors.New("subscription tier conflict")
	ErrSubscriptionTierNotFound   = errors.New("subscription tier not found")
	ErrSubscriptionTierUnavailable = errors.New("subscription tier unavailable")
	ErrSubscriptionPriceChanged   = errors.New("subscription price changed")
	ErrSubscriptionChanged        = errors.New("subscription changed")
	ErrSelfSubscription           = errors.New("self subscription is not allowed")
	ErrSubscriptionPaymentFailed  = errors.New("subscription payment failed")
	ErrSubscriptionTargetNotFound = errors.New("subscription target not found")
	ErrSubscriptionTierValidation = errors.New("subscription tier validation error")
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

type CurrentSubscription struct {
	ID           uuid.UUID
	TierID       uuid.UUID
	TierName     string
	TierStatus   string
	PriceMinor   int64
	Currency     string
	PeriodEndsAt time.Time
}

type SubscriptionPeriod struct {
	PaymentOrderID uuid.UUID
	SubscriptionID uuid.UUID
	BillingPlanID  uuid.UUID
	PeriodStart    time.Time
	PeriodEnd      time.Time
	RevokedAt      *time.Time
}

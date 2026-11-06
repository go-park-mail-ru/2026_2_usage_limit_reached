package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrDonationNotFound     = errors.New("donation not found")
	ErrAuthorNotFound       = errors.New("author not found")
	ErrDonationValidation   = errors.New("donation validation error")
	ErrIdempotencyKeyReused = errors.New("duplicate request_key with order")
	ErrSelfDonation         = errors.New("self donation is not allowed")
)

type Donation struct {
	PaymentOrderID uuid.UUID
	AuthorID       uuid.UUID
	Message        *string
	IsAnonymous    bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

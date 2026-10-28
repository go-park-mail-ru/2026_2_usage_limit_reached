package donationmodels

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrDonationNotFound   = errors.New("donation not found")
	ErrDonationValidation = errors.New("donation validation error")
	ErrSelfDonation       = errors.New("self donation is not allowed")
)

type Donation struct {
	PaymentOrderID uuid.UUID
	AuthorID       uuid.UUID
	Message        *string
	IsAnonymous    bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

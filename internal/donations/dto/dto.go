package dto

import (
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/google/uuid"
)

type CreateDonationRequest struct {
	AuthorID    uuid.UUID `json:"-"`
	RequestKey  string    `json:"request_key"`
	Message     *string   `json:"message"`
	IsAnonymous bool      `json:"is_anonymous"`
	AmountMinor int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
}

func (r CreateDonationRequest) Validate(v *validator.Validator) error {
	var valError string
	switch {
	case r.Message != nil && !v.IsValidDonationMessage(*r.Message):
		valError += "invalid message: "
	case !v.IsValidPaymentAmountMinor(r.AmountMinor):
		valError += "invalid amount: "
	case !v.IsValidPaymentCurrency(r.Currency):
		valError += "invalid currency: "
	case !v.IsValidPaymentReqKey(r.RequestKey):
		valError += "invalid request key: "
	}
	if valError != "" {
		return fmt.Errorf("%s%w", valError, validator.ErrValidation)
	}
	return nil
}

type CreateDonationResponse struct {
	PaymentOrderID uuid.UUID `json:"id"`
	Status         string    `json:"status"`
}

type GetDonationResponse struct {
	PaymentOrderID  uuid.UUID         `json:"id"`
	Message         *string           `json:"message"`
	IsAnonymous     bool              `json:"is_anonymous"`
	AmountMinor     int64             `json:"amount_minor"`
	Currency        string            `json:"currency"`
	Status          string            `json:"status"`
	CreatedOrPaidAt time.Time         `json:"created_or_paid_at"`
	User            *DonationUserInfo `json:"user"`
}

type DonationUserInfo struct {
	Username  string  `json:"username"`
	Nickname  string  `json:"nickname"`
	AvatarUrl *string `json:"avatar_url"`
}

type DonationCursor struct {
	CreatedOrPaidAt time.Time `json:"created_or_paid_at"`
	DonationID      uuid.UUID `json:"donation_id"`
}

type DonationPage feed.Page[*GetDonationResponse, DonationCursor]

type DonationPageResponse struct {
	Items      []*GetDonationResponse `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}

type SentDonation struct {
	PaymentOrderID   uuid.UUID
	PaymentAttemptID *uuid.UUID
	Message          *string
	IsAnonymous      bool
	AmountMinor      int64
	Currency         string
	Status           string
	CreatedAt        time.Time
	Author           *DonationUserInfo
}

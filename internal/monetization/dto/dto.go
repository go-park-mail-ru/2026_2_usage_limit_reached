package dto

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/google/uuid"
)

var ErrDonationAmountBelowMinimum = errors.New("donation_amount_below_minimum")

type CreateDonationRequest struct {
	AuthorID    uuid.UUID `json:"-"`
	RequestKey  string    `json:"request_key"`
	Message     *string   `json:"message"`
	IsAnonymous bool      `json:"is_anonymous"`
	AmountMinor int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
}

func (r CreateDonationRequest) Validate(v *validator.Validator, minimumAmountMinor int64) error {
	errs := make([]error, 0)
	switch {
	case r.Message != nil && !v.IsValidDonationMessage(*r.Message):
		errs = append(errs, fmt.Errorf("invalid message"))
	case !v.IsValidPaymentAmountMinor(r.AmountMinor, minimumAmountMinor):
		errs = append(errs, ErrDonationAmountBelowMinimum)
	case !v.IsValidPaymentCurrency(r.Currency):
		errs = append(errs, fmt.Errorf("invalid currency"))
	case !v.IsValidPaymentReqKey(r.RequestKey):
		errs = append(errs, fmt.Errorf("invalid request key"))
	}
	if len(errs) != 0 {
		errs = append(errs, validator.ErrValidation)
		return errors.Join(errs...)
	}
	return nil
}

type CreateDonationResponse struct {
	PaymentOrderID uuid.UUID `json:"id"`
	Status         string    `json:"status"`
}

type DonationSettings struct {
	MinAmountMinor int64 `json:"min_amount_minor"`
}

type GetDonationResponse struct {
	PaymentOrderID uuid.UUID         `json:"id"`
	Message        *string           `json:"message"`
	IsAnonymous    bool              `json:"is_anonymous"`
	AmountMinor    int64             `json:"amount_minor"`
	Currency       string            `json:"currency"`
	Status         string            `json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
	User           *DonationUserInfo `json:"user"`
}

type DonationUserInfo struct {
	Username  string  `json:"username"`
	Nickname  string  `json:"nickname"`
	AvatarUrl *string `json:"avatar_url"`
}

type DonationCursor struct {
	CreatedAt  time.Time `json:"created_at"`
	DonationID uuid.UUID `json:"donation_id"`
}

type DonationPage feed.Page[*GetDonationResponse, DonationCursor]

type DonationPageResponse struct {
	Items      []*GetDonationResponse `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}

type SentDonation struct {
	PaymentOrderID uuid.UUID
	Message        *string
	IsAnonymous    bool
	AmountMinor    int64
	Currency       string
	Status         string
	CreatedAt      time.Time
	Author         *DonationUserInfo
}

package dto

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/google/uuid"
)

type SubscriptionTier struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	PriceMinor  int64     `json:"price_minor"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status,omitempty"`
}

type SubscriptionTierListResponse struct {
	Items      []*SubscriptionTier `json:"items"`
	NextCursor *string             `json:"next_cursor"`
}

type SubscriptionTierCursor struct {
	PriceMinor int64     `json:"price_minor"`
	TierID     uuid.UUID `json:"tier_id"`
}

type SubscriptionOptionCursor = SubscriptionTierCursor

type CreateSubscriptionTierRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceMinor  int64  `json:"price_minor"`
}

func (r CreateSubscriptionTierRequest) Validate(v *validator.Validator) error {
	errs := make([]error, 0)
	if !v.IsValidSubscriptionTierName(r.Name) {
		errs = append(errs, fmt.Errorf("invalid tier name"))
	}
	if !v.IsValidSubscriptionTierDescription(r.Description) {
		errs = append(errs, fmt.Errorf("invalid tier description"))
	}
	if !v.IsValidPaymentAmountMinor(r.PriceMinor) || !v.IsValidSubscriptionTierPriceMinor(r.PriceMinor) {
		errs = append(errs, fmt.Errorf("invalid tier price"))
	}
	if len(errs) == 0 {
		return nil
	}
	errs = append(errs, validator.ErrValidation)
	return errors.Join(errs...)
}

type UpdateSubscriptionTierRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceMinor  int64  `json:"price_minor"`
	Status      string `json:"status"`
}

func (r UpdateSubscriptionTierRequest) Validate(v *validator.Validator) error {
	errs := make([]error, 0)
	if !v.IsValidSubscriptionTierStatus(r.Status) {
		errs = append(errs, fmt.Errorf("invalid tier status"))
	}
	if !v.IsValidSubscriptionTierName(r.Name) {
		errs = append(errs, fmt.Errorf("invalid tier name"))
	}
	if !v.IsValidSubscriptionTierDescription(r.Description) {
		errs = append(errs, fmt.Errorf("invalid tier description"))
	}
	if !v.IsValidPaymentAmountMinor(r.PriceMinor) || !v.IsValidSubscriptionTierPriceMinor(r.PriceMinor) {
		errs = append(errs, fmt.Errorf("invalid tier price"))
	}
	if len(errs) == 0 {
		return nil
	}
	errs = append(errs, validator.ErrValidation)
	return errors.Join(errs...)
}

type SubscriptionTierReference struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Status string    `json:"status"`
}

type CurrentSubscription struct {
	ID           uuid.UUID                 `json:"id"`
	Tier         SubscriptionTierReference `json:"tier"`
	PriceMinor   int64                     `json:"price_minor"`
	Currency     string                    `json:"currency"`
	PeriodEndsAt time.Time                 `json:"period_ends_at"`
}

type SubscriptionOptionsResponse struct {
	CurrentSubscription *CurrentSubscription `json:"current_subscription"`
	Items               []*SubscriptionTier  `json:"items"`
	NextCursor          *string              `json:"next_cursor"`
}

type SubscriptionTierPage struct {
	CurrentSubscription *CurrentSubscription
	Items               []*SubscriptionTier
	Next                *SubscriptionTierCursor
}

type SubscriptionOptionsPage = SubscriptionTierPage

type CreateSubscriptionRequest struct {
	AuthorID               uuid.UUID  `json:"-"`
	Action                 string     `json:"action"`
	TierID                 uuid.UUID  `json:"tier_id"`
	ExpectedSubscriptionID *uuid.UUID `json:"expected_subscription_id"`
	ExpectedCurrentTierID  *uuid.UUID `json:"expected_current_tier_id"`
	ExpectedPriceMinor     int64      `json:"expected_price_minor"`
	PostID                 *uuid.UUID `json:"post_id"`
	RequestKey             string     `json:"request_key"`
}

func (r CreateSubscriptionRequest) Validate(v *validator.Validator) error {
	errs := make([]error, 0)
	if !v.IsValidSubscriptionAction(r.Action) {
		errs = append(errs, fmt.Errorf("invalid subscription action"))
	}
	if r.TierID == uuid.Nil {
		errs = append(errs, fmt.Errorf("invalid tier id"))
	}
	if !v.IsValidSubscriptionTierPriceMinor(r.ExpectedPriceMinor) {
		errs = append(errs, fmt.Errorf("invalid expected tier price"))
	}
	if !v.IsValidPaymentReqKey(r.RequestKey) {
		errs = append(errs, fmt.Errorf("invalid request key"))
	}
	switch r.Action {
	case "subscribe":
		if r.ExpectedSubscriptionID != nil || r.ExpectedCurrentTierID != nil {
			errs = append(errs, fmt.Errorf("subscribe action cannot have expected subscription ids"))
		}
	case "upgrade":
		if r.ExpectedSubscriptionID == nil || *r.ExpectedSubscriptionID == uuid.Nil ||
			r.ExpectedCurrentTierID == nil || *r.ExpectedCurrentTierID == uuid.Nil {
			errs = append(errs, fmt.Errorf("upgrade action requires expected subscription ids"))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	errs = append(errs, validator.ErrValidation)
	return errors.Join(errs...)
}

type CreateSubscriptionResponse struct {
	CurrentSubscription CurrentSubscription `json:"current_subscription"`
}

type CreateSubscriptionWithPayment struct {
	ReaderID               uuid.UUID
	AuthorID               uuid.UUID
	Action                 string
	TierID                 uuid.UUID
	ExpectedSubscriptionID *uuid.UUID
	ExpectedCurrentTierID  *uuid.UUID
	ExpectedPriceMinor     int64
	PostID                 *uuid.UUID
	RequestKey             string
	ExpiresAt              *time.Time
}

func (r CreateSubscriptionTierRequest) Normalize() CreateSubscriptionTierRequest {
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	return r
}

func (r UpdateSubscriptionTierRequest) Normalize() UpdateSubscriptionTierRequest {
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	return r
}

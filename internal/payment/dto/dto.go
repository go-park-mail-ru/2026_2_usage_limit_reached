package dto

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
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

type CreatePayment struct {
	PayerID     uuid.UUID
	RequestKey  string
	Purpose     string
	Origin      string
	AmountMinor int64
	Currency    string
	ExpiresAt   *time.Time
}

func (p *CreatePayment) Validate(v *validator.Validator) error {
	errs := make([]error, 0)
	switch {
	case !v.IsValidPaymentAmountMinor(p.AmountMinor):
		errs = append(errs, fmt.Errorf("invalid amount"))
	case !v.IsValidPaymentCurrency(p.Currency):
		errs = append(errs, fmt.Errorf("invalid currency"))
	case !v.IsValidPaymentOrigin(p.Origin):
		errs = append(errs, fmt.Errorf("invalid origin"))
	case !v.IsValidPaymentPurpose(p.Purpose):
		errs = append(errs, fmt.Errorf("invalid purpose"))
	case !v.IsValidPaymentReqKey(p.RequestKey):
		errs = append(errs, fmt.Errorf("invalid request key"))
	}
	if len(errs) != 0 {
		errs = append(errs, validator.ErrValidation)
		return errors.Join(errs...)
	}
	return nil
}

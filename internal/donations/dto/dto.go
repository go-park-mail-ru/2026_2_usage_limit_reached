package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateDonationRequest struct {
	AuthorID    uuid.UUID `json:"author_id"`
	RequestKey  string    `json:"request_key"`
	Message     *string   `json:"message"`
	IsAnonymous bool      `json:"is_anonymous"`
	AmountMinor int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
}

type ReceivedDonation struct {
	PaymentOrderID uuid.UUID
	Message        *string
	IsAnonymous    bool
	AmountMinor    int64
	Currency       string
	PaidAt         time.Time
	Donor          *DonationUserInfo
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

type DonationUserInfo struct {
	Username  string
	AvatarKey *string
}

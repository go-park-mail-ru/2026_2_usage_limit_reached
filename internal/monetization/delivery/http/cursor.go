package handlers

import (
	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/cursor"
)

const typeDonations cursor.CursorType = "donations"
const typeSubscriptionTiers cursor.CursorType = "subscription_tiers"

var DonationsCursor = cursor.Codec[dto.DonationCursor]{
	Kind: typeDonations,
	Valid: func(c *dto.DonationCursor) bool {
		return !c.CreatedAt.IsZero() && c.DonationID != uuid.Nil
	},
}

var SubscriptionTiersCursor = cursor.Codec[dto.SubscriptionTierCursor]{
	Kind: typeSubscriptionTiers,
	Valid: func(c *dto.SubscriptionTierCursor) bool {
		return c.PriceMinor > 0 && c.TierID != uuid.Nil
	},
}

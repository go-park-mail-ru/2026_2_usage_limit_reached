package donationhandlers

import (
	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/cursor"
)

const typeDonations cursor.CursorType = "donations"

var DonationsCursor = cursor.Codec[dto.DonationCursor]{
	Kind: typeDonations,
	Valid: func(c *dto.DonationCursor) bool {
		return !c.CreatedOrPaidAt.IsZero() && c.DonationID != uuid.Nil
	},
}

package handlers

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/cursor"
)

const typeHistory cursor.CursorType = "onlineconf_history"

var HistoryCursor = cursor.Codec[dto.HistoryCursor]{
	Kind: typeHistory,
	Valid: func(c *dto.HistoryCursor) bool {
		return c.ID > 0
	},
}

package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAuthorNotFound = errors.New("no author found")
)

type Author struct {
	ID              uuid.UUID
	Bio             string
	Category        *string
	PayoutProvider  *string
	PayoutAccountID *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

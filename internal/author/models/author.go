package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAuthorNotFound = errors.New("no author found")
	ErrAuthorExists   = errors.New("author already exists")
)

type AuthorDetails struct {
	Bio      string
	Category *string
}

type Author struct {
	ID uuid.UUID
	AuthorDetails
	PayoutProvider  *string
	PayoutAccountID *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

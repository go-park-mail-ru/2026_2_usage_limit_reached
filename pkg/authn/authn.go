package authn

import (
	"errors"

	"github.com/google/uuid"
)

var (
	ErrUnavailable  = errors.New("auth service unavailable")
	ErrInvalidToken = errors.New("invalid token")
)

type User struct {
	ID   uuid.UUID
	Role string
}

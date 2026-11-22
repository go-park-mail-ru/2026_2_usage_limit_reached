package identity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type UserInfo struct {
	ID        uuid.UUID
	Username  string
	Nickname  string
	AvatarURL string
	CreatedAt *time.Time
	Status    string
}


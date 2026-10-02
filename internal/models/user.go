package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("no user found")
	ErrInternal     = errors.New("internal server error")
)

type User struct {
	ID           uuid.UUID
	Username     string
	Nickname     string
	Email        string
	PasswordHash string
	AvatarKey    string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

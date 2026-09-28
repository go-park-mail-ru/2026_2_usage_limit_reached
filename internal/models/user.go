package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrValidation         = errors.New("invalid request body")
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

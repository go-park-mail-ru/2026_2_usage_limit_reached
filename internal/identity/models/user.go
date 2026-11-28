package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("no user found")
)

const (
	StatusActive  = "active"
	StatusBlocked = "blocked"
	StatusDeleted = "deleted"
)

const (
	RoleUser    = "user"
	RoleAdmin   = "admin"
	RoleSupport = "support"
)

func IsActiveStatus(status string) bool {
	return status == StatusActive
}

type User struct {
	ID           uuid.UUID
	Username     string
	Nickname     string
	Email        string
	PasswordHash string
	AvatarKey    *string
	Status       string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/google/uuid"
)

var (
	ErrRegistrationFailed = errors.New("registration failed")
	ErrLoginFailed        = errors.New("login failed")
)

type UserProvider interface {
	CreateUser(ctx context.Context, user *models.User) (*models.User, error)
	FindUserByLogin(ctx context.Context, login string) (*models.User, error)
}

type TokenManager interface {
	Generate(ID uuid.UUID, payload any) (string, error)
	TTL() time.Duration
}

type Usecase struct {
	users          UserProvider
	tokenGenerator TokenManager
}

func NewUsecase(u UserProvider, t TokenManager) *Usecase {
	return &Usecase{users: u, tokenGenerator: t}
}

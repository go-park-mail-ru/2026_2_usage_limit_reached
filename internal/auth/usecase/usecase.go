package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/google/uuid"
)

var (
	ErrRegistrationFailed = errors.New("registration failed")
	ErrLoginFailed        = errors.New("login failed")
)

const (
	statusActive = "active"
)

type Repository interface {
	CreateUser(ctx context.Context, user *models.User) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
}

type TokenManager interface {
	Generate(ID uuid.UUID, payload any) (string, error)
	TTL() time.Duration
}

type Usecase struct {
	repo           Repository
	tokenGenerator TokenManager
}

func NewUsecase(r Repository, t TokenManager) *Usecase {
	return &Usecase{repo: r, tokenGenerator: t}
}

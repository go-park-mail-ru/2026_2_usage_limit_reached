package usecase

import (
	"context"
	"errors"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/google/uuid"
)

var (
	ErrUserNotFound    = models.ErrUserNotFound
	ErrUserExists      = models.ErrUserExists
	ErrInvalidNickname = errors.New("invalid nickname")
)

type Repository interface {
	CreateUser(ctx context.Context, user models.User) (*models.User, error)
	UpdateNickname(ctx context.Context, id uuid.UUID, nickname string) (string, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
}

type Usecase struct {
	repo Repository
}

func NewUsecase(r Repository) *Usecase {
	return &Usecase{repo: r}
}

package usecase

import (
	"context"
	"errors"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/models"
	"github.com/google/uuid"
)

var (
	ErrAuthorNotFound  = models.ErrAuthorNotFound
	ErrInvalidBio      = errors.New("invalid bio")
	ErrInvalidCategory = errors.New("invalid category")
)

type Repository interface {
	GetAuthorByID(ctx context.Context, id uuid.UUID) (*models.Author, error)
	UpdateAuthor(ctx context.Context, id uuid.UUID, bio string, category *string) (*models.Author, error)
}

type Usecase struct {
	repo Repository
}

func NewUsecase(r Repository) *Usecase {
	return &Usecase{repo: r}
}

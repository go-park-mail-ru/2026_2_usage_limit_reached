package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/models"
	"github.com/google/uuid"
)

func (uc *Usecase) FindAuthorByID(ctx context.Context, authorID uuid.UUID) (*dto.AuthorInfo, error) {
	author, err := uc.repo.GetAuthorByID(ctx, authorID)
	if err != nil {
		if errors.Is(err, models.ErrAuthorNotFound) {
			return nil, fmt.Errorf("find author: %w", ErrAuthorNotFound)
		}
		return nil, fmt.Errorf("find author: %w", err)
	}

	return dto.ToAuthorInfo(author), nil
}

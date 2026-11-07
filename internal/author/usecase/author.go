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

func (uc *Usecase) UpdateAuthor(ctx context.Context, req dto.UpdateAuthorRequest) (*dto.AuthorInfo, error) {
	req = req.Normalize()
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("update author: %w", err)
	}

	author, err := uc.repo.UpdateAuthor(ctx, req.AuthorID, models.AuthorDetails{
		Bio:      req.Bio,
		Category: req.Category,
	})
	if err != nil {
		if errors.Is(err, models.ErrAuthorNotFound) {
			return nil, fmt.Errorf("update author: %w", ErrAuthorNotFound)
		}
		return nil, fmt.Errorf("update author: %w", err)
	}

	return dto.ToAuthorInfo(author), nil
}

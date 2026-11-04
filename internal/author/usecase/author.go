package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	"github.com/google/uuid"
)

func (uc *Usecase) FindAuthorByID(ctx context.Context, authorID uuid.UUID) (*dto.AuthorInfo, error) {
	author, err := uc.repo.GetAuthorByID(ctx, authorID)
	if err != nil {
		return nil, fmt.Errorf("find author: %w", err)
	}

	return dto.ToAuthorInfo(author), nil
}

func (uc *Usecase) UpdateAuthor(ctx context.Context, authorID uuid.UUID, bio string, category *string) (*dto.AuthorInfo, error) {
	bio = strings.TrimSpace(bio)
	if bio == "" {
		return nil, fmt.Errorf("update author: %w", ErrInvalidBio)
	}

	if category != nil {
		trimmedCategory := strings.TrimSpace(*category)
		if trimmedCategory == "" {
			return nil, fmt.Errorf("update author: %w", ErrInvalidCategory)
		}

		category = &trimmedCategory
	}

	author, err := uc.repo.UpdateAuthor(ctx, authorID, bio, category)
	if err != nil {
		return nil, fmt.Errorf("update author: %w", err)
	}

	return dto.ToAuthorInfo(author), nil
}

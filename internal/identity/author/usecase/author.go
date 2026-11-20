package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/models"
	"github.com/google/uuid"
)

func (uc *Usecase) FindAuthorByID(ctx context.Context, authorID uuid.UUID) (*authordto.AuthorInfo, error) {
	author, err := uc.repo.GetAuthorByID(ctx, authorID)
	if err != nil {
		if errors.Is(err, models.ErrAuthorNotFound) {
			return nil, fmt.Errorf("find author: %w", ErrAuthorNotFound)
		}
		return nil, fmt.Errorf("find author: %w", err)
	}

	return authordto.ToAuthorInfo(author), nil
}

func (uc *Usecase) FindAuthorsByIDs(ctx context.Context, ids []uuid.UUID) ([]*authordto.AuthorInfo, error) {
	if len(ids) == 0 {
		return []*authordto.AuthorInfo{}, nil
	}

	authors, err := uc.repo.GetAuthorsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("find authors: %w", err)
	}

	infos := make([]*authordto.AuthorInfo, 0, len(authors))
	for i := range authors {
		infos = append(infos, authordto.ToAuthorInfo(&authors[i]))
	}

	return infos, nil
}

func (uc *Usecase) CreateAuthor(ctx context.Context, req authordto.UpdateAuthorRequest) (*authordto.AuthorInfo, error) {
	req = req.Normalize()
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("create author: %w", err)
	}

	author, err := uc.repo.CreateAuthor(ctx, req.AuthorID, models.AuthorDetails{
		Bio:      req.Bio,
		Category: req.Category,
	})
	if err != nil {
		if errors.Is(err, models.ErrAuthorExists) {
			return uc.FindAuthorByID(ctx, req.AuthorID)
		}
		return nil, fmt.Errorf("create author: %w", err)
	}

	return authordto.ToAuthorInfo(author), nil
}

func (uc *Usecase) UpdateAuthor(ctx context.Context, req authordto.UpdateAuthorRequest) (*authordto.AuthorInfo, error) {
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

	return authordto.ToAuthorInfo(author), nil
}

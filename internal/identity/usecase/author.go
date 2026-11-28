package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/mediaurl"
	"github.com/google/uuid"
)

const AuthorSearchPageSize = 20

var authorRepoErrors = []struct{ from, to error }{
	{models.ErrAuthorNotFound, ErrAuthorNotFound},
}

func mapAuthorRepoError(op string, err error) error {
	for _, e := range authorRepoErrors {
		if errors.Is(err, e.from) {
			return fmt.Errorf("%s: %w", op, e.to)
		}
	}

	return fmt.Errorf("%s: %w", op, err)
}

func (uc *Usecase) SearchAuthors(ctx context.Context, req dto.AuthorSearchRequest) (*dto.AuthorSearchPage, error) {
	if req.Query == "" && len(req.Categories) == 0 {
		return nil, fmt.Errorf("search authors: empty query and categories: %w", validator.ErrValidation)
	}

	authors, err := uc.repo.SearchAuthors(ctx, req.Query, req.Categories, req.Cursor, AuthorSearchPageSize+1)
	if err != nil {
		return nil, fmt.Errorf("search authors: %w", err)
	}

	items := make([]*dto.AuthorSearchItem, 0, len(authors))
	for _, author := range authors {
		items = append(items, dto.ToAuthorSearchItem(author, mediaurl.PublicPtr(uc.avatarBasePath, author.AvatarKey)))
	}

	page := feed.Paginate(items, AuthorSearchPageSize, func(card *dto.AuthorSearchItem) dto.AuthorSearchCursor {
		return dto.AuthorSearchCursor{Nickname: strings.ToLower(card.Nickname), ID: card.ID}
	})

	return &dto.AuthorSearchPage{Items: page.Items, Next: page.Next}, nil
}

func (uc *Usecase) CreateAuthor(ctx context.Context, userID uuid.UUID, req dto.AuthorCreateRequest) (*dto.AuthorMeResponse, error) {
	if _, err := uc.getActiveUser(ctx, userID); err != nil {
		return nil, fmt.Errorf("create author: %w", err)
	}

	author, err := uc.repo.CreateAuthor(ctx, userID, req.Bio, req.Category)
	if err != nil {
		return nil, mapAuthorRepoError("create author", err)
	}

	return dto.ToAuthorMeResponse(author), nil
}

func (uc *Usecase) GetAuthor(ctx context.Context, userID uuid.UUID) (*dto.AuthorMeResponse, error) {
	if _, err := uc.getActiveUser(ctx, userID); err != nil {
		return nil, fmt.Errorf("get author: %w", err)
	}

	author, err := uc.repo.GetAuthorByID(ctx, userID)
	if err != nil {
		return nil, mapAuthorRepoError("get author", err)
	}

	return dto.ToAuthorMeResponse(author), nil
}

func (uc *Usecase) UpdateAuthor(ctx context.Context, userID uuid.UUID, req dto.AuthorCreateRequest) (*dto.AuthorMeResponse, error) {
	if _, err := uc.getActiveUser(ctx, userID); err != nil {
		return nil, fmt.Errorf("update author: %w", err)
	}

	author, err := uc.repo.UpdateAuthorDetails(ctx, userID, req.Bio, req.Category)
	if err != nil {
		return nil, mapAuthorRepoError("update author", err)
	}

	return dto.ToAuthorMeResponse(author), nil
}

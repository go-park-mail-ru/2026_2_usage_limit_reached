package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	identitycontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/identity"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	usecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMonetizationUsecase_GetMySubscriptionTiers(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	tierIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	archivedAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	cursor := &dto.SubscriptionTierCursor{PriceMinor: 1200, TierID: tierIDs[0]}
	tiers := []*models.SubscriptionTier{
		{ID: tierIDs[0], Name: "Bronze", Description: "Bronze tier", PriceMinor: 1200, Currency: "RUB"},
		{ID: tierIDs[1], Name: "Silver", Description: "Silver tier", PriceMinor: 2400, Currency: "RUB", ArchivedAt: &archivedAt},
		{ID: tierIDs[2], Name: "Gold", Description: "Gold tier", PriceMinor: 3600, Currency: "RUB"},
	}

	uc, repo, _, _, _ := newUsecase(t)
	repo.On("GetSubscriptionTiers", mock.Anything, authorID, cursor, pageLimit+1).
		Return(tiers, nil).Once()

	got, err := uc.GetMySubscriptionTiers(context.Background(), authorID, cursor, pageLimit)

	require.NoError(t, err)
	require.Equal(t, &dto.SubscriptionTierPage{
		Items: []*dto.SubscriptionTier{
			{ID: tierIDs[0], Name: "Bronze", Description: "Bronze tier", PriceMinor: 1200, Currency: "RUB", Status: "open"},
			{ID: tierIDs[1], Name: "Silver", Description: "Silver tier", PriceMinor: 2400, Currency: "RUB", Status: "archived"},
		},
		Next: &dto.SubscriptionTierCursor{PriceMinor: 2400, TierID: tierIDs[1]},
	}, got)
}

func TestMonetizationUsecase_GetMySubscriptionTiers_RepositoryError(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	uc, repo, _, _, _ := newUsecase(t)
	repo.On("GetSubscriptionTiers", mock.Anything, authorID, (*dto.SubscriptionTierCursor)(nil), pageLimit+1).
		Return(nil, technicalErr).Once()

	got, err := uc.GetMySubscriptionTiers(context.Background(), authorID, nil, pageLimit)

	require.Nil(t, got)
	require.ErrorIs(t, err, technicalErr)
}

func TestMonetizationUsecase_GetSubscriptionOptions_InactiveAuthorNotFound(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	uc, _, _, _, identity := newUsecase(t)
	identity.On("GetUsers", mock.Anything, []uuid.UUID{authorID}).
		Return(map[uuid.UUID]*identitycontracts.UserInfo{
			authorID: {ID: authorID, Status: "deleted"},
		}, nil).Once()

	got, err := uc.GetSubscriptionOptions(context.Background(), uuid.Nil, authorID, nil, nil, pageLimit)

	require.Nil(t, got)
	require.ErrorIs(t, err, usecase.ErrNotFound)
}

func TestMonetizationUsecase_UpdateSubscriptionTier(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	tierID := uuid.New()
	archivedAt := time.Now()
	req := dto.UpdateSubscriptionTierRequest{
		Name:        "Bronze",
		Description: "Bronze tier",
		PriceMinor:  50000,
		Status:      "archived",
	}
	tier := &models.SubscriptionTier{
		ID:          tierID,
		AuthorID:    authorID,
		Name:        req.Name,
		Description: req.Description,
		PriceMinor:  req.PriceMinor,
		Currency:    "RUB",
		ArchivedAt:  &archivedAt,
	}

	uc, repo, _, _, identity := newUsecase(t)
	identity.On("GetUsers", mock.Anything, []uuid.UUID{authorID}).
		Return(activeIdentityAuthor(authorID), nil).Twice()
	repo.On("UpdateSubscriptionTier", mock.Anything, authorID, tierID, req).
		Return(tier, nil).Once()

	got, err := uc.UpdateSubscriptionTier(context.Background(), authorID, tierID, req)

	require.NoError(t, err)
	require.Equal(t, &dto.SubscriptionTier{
		ID:          tierID,
		Name:        req.Name,
		Description: req.Description,
		PriceMinor:  req.PriceMinor,
		Currency:    "RUB",
		Status:      "archived",
	}, got)
}

func TestMonetizationUsecase_UpdateSubscriptionTier_Validation(t *testing.T) {
	t.Parallel()

	uc, _, _, _, _ := newUsecase(t)
	got, err := uc.UpdateSubscriptionTier(context.Background(), uuid.New(), uuid.New(), dto.UpdateSubscriptionTierRequest{
		Name:        "x",
		Description: "Bronze tier",
		PriceMinor:  50000,
		Status:      "open",
	})

	require.Nil(t, got)
	require.ErrorIs(t, err, usecase.ErrValidation)
}

func TestMonetizationUsecase_UpdateSubscriptionTier_Conflict(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	tierID := uuid.New()
	req := dto.UpdateSubscriptionTierRequest{
		Name:        "Bronze",
		Description: "Bronze tier",
		PriceMinor:  50000,
		Status:      "open",
	}
	uc, repo, _, _, identity := newUsecase(t)
	identity.On("GetUsers", mock.Anything, []uuid.UUID{authorID}).
		Return(activeIdentityAuthor(authorID), nil).Once()
	repo.On("UpdateSubscriptionTier", mock.Anything, authorID, tierID, req).
		Return(nil, models.ErrSubscriptionTierConflict).Once()

	got, err := uc.UpdateSubscriptionTier(context.Background(), authorID, tierID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, usecase.ErrConflict)
	require.ErrorIs(t, err, models.ErrSubscriptionTierConflict)
}

func TestMonetizationUsecase_CreateSubscriptionTier(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	tierID := uuid.New()
	createdAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	req := dto.CreateSubscriptionTierRequest{
		Name:        "Bronze",
		Description: "Bronze tier",
		PriceMinor:  50000,
	}
	tier := &models.SubscriptionTier{
		ID:          tierID,
		AuthorID:    authorID,
		Name:        req.Name,
		Description: req.Description,
		PriceMinor:  req.PriceMinor,
		Currency:    "RUB",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}

	uc, repo, _, _, identity := newUsecase(t)
	identity.On("GetUsers", mock.Anything, []uuid.UUID{authorID}).
		Return(activeIdentityAuthor(authorID), nil).Twice()
	repo.On("CreateSubscriptionTier", mock.Anything, authorID, req).
		Return(tier, nil).Once()

	got, err := uc.CreateSubscriptionTier(context.Background(), authorID, req)

	require.NoError(t, err)
	require.Equal(t, &dto.SubscriptionTier{
		ID:          tierID,
		Name:        req.Name,
		Description: req.Description,
		PriceMinor:  req.PriceMinor,
		Currency:    "RUB",
		Status:      "open",
	}, got)
}

func TestMonetizationUsecase_CreateSubscriptionTier_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		author uuid.UUID
		req    dto.CreateSubscriptionTierRequest
	}{
		{
			name: "nil author",
			req:  dto.CreateSubscriptionTierRequest{Name: "Bronze", Description: "Bronze tier", PriceMinor: 50000},
		},
		{
			name:   "invalid request",
			author: uuid.New(),
			req:    dto.CreateSubscriptionTierRequest{Name: "Bronze", Description: "Bronze tier"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, _, _, _, _ := newUsecase(t)
			got, err := uc.CreateSubscriptionTier(context.Background(), tt.author, tt.req)

			require.Nil(t, got)
			require.ErrorIs(t, err, usecase.ErrValidation)
		})
	}
}

func TestMonetizationUsecase_CreateSubscriptionTier_RepositoryErrors(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	req := dto.CreateSubscriptionTierRequest{Name: "Bronze", Description: "Bronze tier", PriceMinor: 50000}
	uc, repo, _, _, identity := newUsecase(t)
	identity.On("GetUsers", mock.Anything, []uuid.UUID{authorID}).
		Return(activeIdentityAuthor(authorID), nil).Once()
	repo.On("CreateSubscriptionTier", mock.Anything, authorID, req).
		Return(nil, models.ErrSubscriptionTierConflict).Once()

	got, err := uc.CreateSubscriptionTier(context.Background(), authorID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, usecase.ErrConflict)
	require.ErrorIs(t, err, models.ErrSubscriptionTierConflict)
}

func TestMonetizationUsecase_CreateSubscriptionTier_RepositoryFailure(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	req := dto.CreateSubscriptionTierRequest{Name: "Bronze", Description: "Bronze tier", PriceMinor: 50000}
	repoErr := errors.New("storage unavailable")
	uc, repo, _, _, identity := newUsecase(t)
	identity.On("GetUsers", mock.Anything, []uuid.UUID{authorID}).
		Return(activeIdentityAuthor(authorID), nil).Once()
	repo.On("CreateSubscriptionTier", mock.Anything, authorID, req).
		Return(nil, repoErr).Once()

	got, err := uc.CreateSubscriptionTier(context.Background(), authorID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, repoErr)
}

func TestMonetizationUsecase_CreateSubscriptionTier_RechecksAuthorStatus(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	tierID := uuid.New()
	req := dto.CreateSubscriptionTierRequest{
		Name:        "Bronze",
		Description: "Bronze tier",
		PriceMinor:  50000,
	}
	tier := &models.SubscriptionTier{ID: tierID}
	uc, repo, _, _, identity := newUsecase(t)
	identity.On("GetUsers", mock.Anything, []uuid.UUID{authorID}).
		Return(activeIdentityAuthor(authorID), nil).Once()
	repo.On("CreateSubscriptionTier", mock.Anything, authorID, req).
		Return(tier, nil).Once()
	identity.On("GetUsers", mock.Anything, []uuid.UUID{authorID}).
		Return(inactiveIdentityAuthor(authorID), nil).Once()

	got, err := uc.CreateSubscriptionTier(context.Background(), authorID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, usecase.ErrNotFound)
}

func activeIdentityAuthor(authorID uuid.UUID) map[uuid.UUID]*identitycontracts.UserInfo {
	return map[uuid.UUID]*identitycontracts.UserInfo{
		authorID: {ID: authorID, Status: "active"},
	}
}

func inactiveIdentityAuthor(authorID uuid.UUID) map[uuid.UUID]*identitycontracts.UserInfo {
	return map[uuid.UUID]*identitycontracts.UserInfo{
		authorID: {ID: authorID, Status: "deleted"},
	}
}

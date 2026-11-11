package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/usecase/mock_author_usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var authorID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

func ptr(s string) *string { return &s }

func newAuthor(bio string, category *string) *models.Author {
	return &models.Author{
		ID:            authorID,
		AuthorDetails: models.AuthorDetails{Bio: bio, Category: category},
	}
}

func TestUsecase_FindAuthorByID(t *testing.T) {
	repoErr := errors.New("db down")

	tests := []struct {
		name         string
		mockBehavior func(m *mock_author_usecase.MockRepository)
		expected     *dto.AuthorInfo
		expectedErr  error
	}{
		{
			name: "Success",
			mockBehavior: func(m *mock_author_usecase.MockRepository) {
				m.EXPECT().GetAuthorByID(mock.Anything, authorID).Return(newAuthor("bio", ptr("tech")), nil)
			},
			expected: &dto.AuthorInfo{ID: authorID, Bio: "bio", Category: ptr("tech")},
		},
		{
			name: "Author not found",
			mockBehavior: func(m *mock_author_usecase.MockRepository) {
				m.EXPECT().GetAuthorByID(mock.Anything, authorID).Return(nil, models.ErrAuthorNotFound)
			},
			expectedErr: usecase.ErrAuthorNotFound,
		},
		{
			name: "Repository error",
			mockBehavior: func(m *mock_author_usecase.MockRepository) {
				m.EXPECT().GetAuthorByID(mock.Anything, authorID).Return(nil, repoErr)
			},
			expectedErr: repoErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mock_author_usecase.NewMockRepository(t)
			tt.mockBehavior(repo)
			uc := usecase.NewUsecase(repo)

			got, err := uc.FindAuthorByID(context.Background(), authorID)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Nil(t, got)
				if !errors.Is(tt.expectedErr, usecase.ErrAuthorNotFound) {
					require.NotErrorIs(t, err, usecase.ErrAuthorNotFound)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestUsecase_UpdateAuthor(t *testing.T) {
	repoErr := errors.New("db down")

	tests := []struct {
		name         string
		req          dto.UpdateAuthorRequest
		mockBehavior func(m *mock_author_usecase.MockRepository)
		expected     *dto.AuthorInfo
		expectedErr  error
	}{
		{
			name: "Success trims bio and category",
			req:  dto.UpdateAuthorRequest{AuthorID: authorID, Bio: "  bio  ", Category: ptr("  tech  ")},
			mockBehavior: func(m *mock_author_usecase.MockRepository) {
				m.EXPECT().
					UpdateAuthor(mock.Anything, authorID, models.AuthorDetails{Bio: "bio", Category: ptr("tech")}).
					Return(newAuthor("bio", ptr("tech")), nil)
			},
			expected: &dto.AuthorInfo{ID: authorID, Bio: "bio", Category: ptr("tech")},
		},
		{
			name: "Success without category",
			req:  dto.UpdateAuthorRequest{AuthorID: authorID, Bio: "bio"},
			mockBehavior: func(m *mock_author_usecase.MockRepository) {
				m.EXPECT().
					UpdateAuthor(mock.Anything, authorID, models.AuthorDetails{Bio: "bio"}).
					Return(newAuthor("bio", nil), nil)
			},
			expected: &dto.AuthorInfo{ID: authorID, Bio: "bio"},
		},
		{
			name:         "Empty bio",
			req:          dto.UpdateAuthorRequest{AuthorID: authorID, Bio: "   "},
			mockBehavior: func(m *mock_author_usecase.MockRepository) {},
			expectedErr:  validator.ErrValidation,
		},
		{
			name:         "Empty category",
			req:          dto.UpdateAuthorRequest{AuthorID: authorID, Bio: "bio", Category: ptr("   ")},
			mockBehavior: func(m *mock_author_usecase.MockRepository) {},
			expectedErr:  validator.ErrValidation,
		},
		{
			name: "Author not found",
			req:  dto.UpdateAuthorRequest{AuthorID: authorID, Bio: "bio"},
			mockBehavior: func(m *mock_author_usecase.MockRepository) {
				m.EXPECT().UpdateAuthor(mock.Anything, authorID, mock.Anything).Return(nil, models.ErrAuthorNotFound)
			},
			expectedErr: usecase.ErrAuthorNotFound,
		},
		{
			name: "Repository error",
			req:  dto.UpdateAuthorRequest{AuthorID: authorID, Bio: "bio"},
			mockBehavior: func(m *mock_author_usecase.MockRepository) {
				m.EXPECT().UpdateAuthor(mock.Anything, authorID, mock.Anything).Return(nil, repoErr)
			},
			expectedErr: repoErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mock_author_usecase.NewMockRepository(t)
			tt.mockBehavior(repo)
			uc := usecase.NewUsecase(repo)

			got, err := uc.UpdateAuthor(context.Background(), tt.req)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

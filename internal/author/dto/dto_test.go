package dto_test

import (
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var authorID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

func ptr(s string) *string { return &s }

func TestUpdateAuthorRequest_Normalize(t *testing.T) {
	t.Run("Trims bio and category", func(t *testing.T) {
		got := dto.UpdateAuthorRequest{AuthorID: authorID, Bio: "  bio  ", Category: ptr("  tech  ")}.Normalize()

		require.Equal(t, "bio", got.Bio)
		require.Equal(t, "tech", *got.Category)
		require.Equal(t, authorID, got.AuthorID)
	})

	t.Run("Keeps nil category", func(t *testing.T) {
		got := dto.UpdateAuthorRequest{Bio: " bio "}.Normalize()

		require.Equal(t, "bio", got.Bio)
		require.Nil(t, got.Category)
	})
}

func TestUpdateAuthorRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     dto.UpdateAuthorRequest
		wantErr bool
	}{
		{name: "Valid", req: dto.UpdateAuthorRequest{Bio: "bio", Category: ptr("tech")}},
		{name: "Valid without category", req: dto.UpdateAuthorRequest{Bio: "bio"}},
		{name: "Empty bio", req: dto.UpdateAuthorRequest{Bio: ""}, wantErr: true},
		{name: "Empty category", req: dto.UpdateAuthorRequest{Bio: "bio", Category: ptr("")}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()

			if tt.wantErr {
				require.ErrorIs(t, err, validator.ErrValidation)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestToAuthorInfo(t *testing.T) {
	t.Run("Nil author", func(t *testing.T) {
		require.Nil(t, dto.ToAuthorInfo(nil))
	})

	t.Run("Mapping", func(t *testing.T) {
		got := dto.ToAuthorInfo(&models.Author{
			ID:            authorID,
			AuthorDetails: models.AuthorDetails{Bio: "bio", Category: ptr("tech")},
		})

		require.Equal(t, &dto.AuthorInfo{ID: authorID, Bio: "bio", Category: ptr("tech")}, got)
	})
}

package dto_test

import (
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	profileUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	profileTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

func newProfile() *models.User {
	return &models.User{
		ID:        profileUserID,
		Username:  "ivan001",
		Nickname:  "Ivan",
		Email:     "ivan@gmail.com",
		CreatedAt: profileTime,
	}
}

func TestUpdateProfileRequest_Validate(t *testing.T) {
	v := testutils.SetupValidator()

	tests := []struct {
		name    string
		req     dto.UpdateProfileRequest
		wantErr bool
	}{
		{name: "Valid", req: dto.UpdateProfileRequest{Nickname: "Nick"}},
		{name: "Empty nickname", req: dto.UpdateProfileRequest{Nickname: "  "}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate(v)

			if tt.wantErr {
				require.ErrorIs(t, err, validator.ErrValidation)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestToProfileResponse(t *testing.T) {
	avatar := "/media/avatars/ivan.png"

	t.Run("Nil profile", func(t *testing.T) {
		require.Nil(t, dto.ToProfileResponse(nil, &avatar))
	})

	t.Run("Profile without avatar", func(t *testing.T) {
		got := dto.ToProfileResponse(newProfile(), nil)

		require.Equal(t, "ivan@gmail.com", got.User.Email)
		require.Equal(t, "ivan001", got.User.Username)
		require.Nil(t, got.User.AvatarURL)
	})

	t.Run("Profile with avatar", func(t *testing.T) {
		got := dto.ToProfileResponse(newProfile(), &avatar)

		require.Equal(t, profileUserID, got.User.ID)
		require.Equal(t, profileTime, got.User.CreatedAt)
		require.Equal(t, &avatar, got.User.AvatarURL)
	})
}

func TestToPublicProfileResponse(t *testing.T) {
	avatar := "/media/avatars/ivan.png"

	t.Run("Nil profile", func(t *testing.T) {
		require.Nil(t, dto.ToPublicProfileResponse(nil, &avatar))
	})

	t.Run("Public response has no email", func(t *testing.T) {
		got := dto.ToPublicProfileResponse(newProfile(), &avatar)

		require.Equal(t, dto.PublicProfileUserResponse{
			ID:        profileUserID,
			Username:  "ivan001",
			Nickname:  "Ivan",
			AvatarURL: &avatar,
			CreatedAt: profileTime,
		}, got.User)
	})
}

func TestToAvatarResponse(t *testing.T) {
	require.Equal(t, &dto.AvatarResponse{AvatarURL: "/media/a.png"}, dto.ToAvatarResponse("/media/a.png"))
}

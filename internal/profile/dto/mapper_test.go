package dto_test

import (
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	profileUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	profileTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

func newProfile(author *models.ProfileAuthor) *models.Profile {
	return &models.Profile{
		UserID:    profileUserID,
		Username:  "ivan001",
		Nickname:  "Ivan",
		Email:     "ivan@gmail.com",
		CreatedAt: profileTime,
		Author:    author,
	}
}

func TestToProfileResponse(t *testing.T) {
	avatar := "/media/avatars/ivan.png"

	t.Run("Nil profile", func(t *testing.T) {
		require.Nil(t, dto.ToProfileResponse(nil, &avatar))
	})

	t.Run("Reader without author", func(t *testing.T) {
		got := dto.ToProfileResponse(newProfile(nil), nil)

		require.Equal(t, "ivan@gmail.com", got.User.Email)
		require.Equal(t, "ivan001", got.User.Username)
		require.Nil(t, got.User.AvatarURL)
		require.Nil(t, got.Author)
	})

	t.Run("Author with avatar", func(t *testing.T) {
		got := dto.ToProfileResponse(newProfile(&models.ProfileAuthor{Bio: "bio", Category: ptr("tech")}), &avatar)

		require.Equal(t, profileUserID, got.User.ID)
		require.Equal(t, profileTime, got.User.CreatedAt)
		require.Equal(t, &avatar, got.User.AvatarURL)
		require.Equal(t, &dto.AuthorResponse{Bio: "bio", Category: ptr("tech")}, got.Author)
	})
}

func TestToPublicProfileResponse(t *testing.T) {
	avatar := "/media/avatars/ivan.png"

	t.Run("Nil profile", func(t *testing.T) {
		require.Nil(t, dto.ToPublicProfileResponse(nil, &avatar))
	})

	t.Run("Public response has no email", func(t *testing.T) {
		got := dto.ToPublicProfileResponse(newProfile(&models.ProfileAuthor{Bio: "bio"}), &avatar)

		require.Equal(t, dto.PublicProfileUserResponse{
			ID:        profileUserID,
			Username:  "ivan001",
			Nickname:  "Ivan",
			AvatarURL: &avatar,
			CreatedAt: profileTime,
		}, got.User)
		require.Equal(t, &dto.AuthorResponse{Bio: "bio"}, got.Author)
	})

	t.Run("Without author", func(t *testing.T) {
		require.Nil(t, dto.ToPublicProfileResponse(newProfile(nil), nil).Author)
	})
}

func TestToAvatarResponse(t *testing.T) {
	require.Equal(t, &dto.AvatarResponse{AvatarURL: "/media/a.png"}, dto.ToAvatarResponse("/media/a.png"))
}

package dto_test

import (
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	userID    = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	createdAt = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	updatedAt = createdAt.Add(time.Hour)
)

func newUser() *models.User {
	return &models.User{
		ID:           userID,
		Username:     "ivan001",
		Nickname:     "Ivan",
		Email:        "ivan@gmail.com",
		PasswordHash: "hash",
		Status:       models.StatusActive,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func TestIsActiveStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{status: models.StatusActive, expected: true},
		{status: models.StatusBlocked, expected: false},
		{status: models.StatusDeleted, expected: false},
		{status: "", expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			require.Equal(t, tt.expected, dto.IsActiveStatus(tt.status))
			require.Equal(t, tt.expected, dto.UserInfo{Status: tt.status}.IsActive())
		})
	}
}

func TestToUserInfo(t *testing.T) {
	avatar := "/media/avatars/ivan.png"

	t.Run("Nil user", func(t *testing.T) {
		require.Nil(t, dto.ToUserInfo(nil, &avatar))
	})

	t.Run("Mapping", func(t *testing.T) {
		got := dto.ToUserInfo(newUser(), &avatar)

		require.Equal(t, &dto.UserInfo{
			ID:        userID,
			Email:     "ivan@gmail.com",
			Username:  "ivan001",
			Nickname:  "Ivan",
			AvatarURL: &avatar,
			Status:    models.StatusActive,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}, got)
	})

	t.Run("Without avatar", func(t *testing.T) {
		require.Nil(t, dto.ToUserInfo(newUser(), nil).AvatarURL)
	})
}

func TestToUserCredentials(t *testing.T) {
	avatar := "/media/avatars/ivan.png"

	t.Run("Nil user", func(t *testing.T) {
		require.Nil(t, dto.ToUserCredentials(nil, &avatar))
	})

	t.Run("Mapping", func(t *testing.T) {
		got := dto.ToUserCredentials(newUser(), &avatar)

		require.Equal(t, "hash", got.PasswordHash)
		require.Equal(t, userID, got.ID)
		require.Equal(t, &avatar, got.AvatarURL)
		require.True(t, got.IsActive())
	})
}

package memory

import (
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	"github.com/google/uuid"
)

func NewUserRepositoryWithMockData(users []models.User) (*UserRepository, error) {
	repo := NewUserRepository()
	for _, user := range users {
		if user.ID == uuid.Nil || user.Email == "" || user.Username == "" {
			return nil, fmt.Errorf("mock user has an empty ID, email, or username")
		}
		if _, exists := repo.users[user.ID]; exists {
			return nil, fmt.Errorf("duplicate mock user ID %s", user.ID)
		}
		if _, exists := repo.byEmail[user.Email]; exists {
			return nil, fmt.Errorf("duplicate mock user email %s", user.Email)
		}
		if _, exists := repo.byUsername[user.Username]; exists {
			return nil, fmt.Errorf("duplicate mock username %s", user.Username)
		}
		repo.users[user.ID] = user
		repo.byEmail[user.Email] = user.ID
		repo.byUsername[user.Username] = user.ID
	}
	return repo, nil
}

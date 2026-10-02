package memory

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
)

type UserRepository struct {
	mu         sync.RWMutex
	users      map[uuid.UUID]models.User
	byEmail    map[string]uuid.UUID
	byUsername map[string]uuid.UUID
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		users:      make(map[uuid.UUID]models.User),
		byEmail:    make(map[string]uuid.UUID),
		byUsername: make(map[string]uuid.UUID),
	}
}

func (r *UserRepository) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byEmail[u.Email]; ok {
		return nil, models.ErrUserExists
	}
	if _, ok := r.byUsername[u.Username]; ok {
		return nil, models.ErrUserExists
	}
	u.ID = uuid.New()
	r.users[u.ID] = *u
	r.byEmail[u.Email] = u.ID
	r.byUsername[u.Username] = u.ID
	return u, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[id]
	if !ok {
		return nil, models.ErrUserNotFound
	}
	return &user, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, login string) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.byEmail {
		if r.users[u].Email == login {
			user := r.users[u]
			return &user, nil
		}
	}
	return nil, models.ErrUserNotFound
}

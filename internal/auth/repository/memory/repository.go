package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
)

type UserRepository struct {
	mu         sync.RWMutex
	users      map[uuid.UUID]models.User
	byEmail    map[string]uuid.UUID
	byUsername map[string]uuid.UUID
}

func NewUserRepository() *UserRepository {
	ivanID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	alexandraID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	petrID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

	createdAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	const passwordHash = "$2b$10$/AesN2sVYvp4.KICRTYjZuGlIA49n6QuSgRXD7tw3p3teR7ghsdeC"

	return &UserRepository{
		users: map[uuid.UUID]models.User{
			ivanID: {
				ID: ivanID,
				Username: "ivan001",
				Nickname: "Иван",
				Email: "ivan@gmail.com",
				PasswordHash: passwordHash,
				Status: "active",
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
			alexandraID: {
				ID: alexandraID,
				Username: "alexandra",
				Nickname: "Александра",
				Email: "sasha@gmail.com",
				PasswordHash: passwordHash,
				Status: "active",
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
			petrID: {
				ID: petrID,
				Username: "petr",
				Nickname: "Петр",
				Email: "petr@gmail.com",
				PasswordHash: passwordHash,
				Status: "active",
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
		},

		byEmail: map[string]uuid.UUID{
			"ivan@gmail.com":  ivanID,
			"sasha@gmail.com": alexandraID,
			"petr@gmail.com":  petrID,
		},
		
		byUsername: map[string]uuid.UUID{
			"ivan001":   ivanID,
			"alexandra": alexandraID,
			"petr":      petrID,
		},
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

func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	userID, ok := r.byUsername[username]
	if !ok {
		return nil, models.ErrUserNotFound
	}

	user, ok := r.users[userID]
	if !ok {
		return nil, models.ErrUserNotFound
	}

	return &user, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	userID, ok := r.byEmail[email]
	if !ok {
		return nil, models.ErrUserNotFound
	}

	user, ok := r.users[userID]
	if !ok {
		return nil, models.ErrUserNotFound
	}

	return &user, nil
}

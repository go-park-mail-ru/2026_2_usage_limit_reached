package main

import (
	"context"
	"fmt"
	"time"

	authmodels "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	profilemodels "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	authrepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/repository/memory"
	profilerepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/repository/memory"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const password = "DemoPassword123!"

func newSeededRepos(ctx context.Context) (*authrepo.UserRepository, *profilerepo.ProfileRepository, error) {
	userRepo := authrepo.NewUserRepository()
	userIdcs, err := seedUsers(ctx, userRepo)
	if err != nil {
		return nil, nil, fmt.Errorf("seed user repo: %w", err)
	}

	profileRepo, err := profilerepo.NewProfileRepositoryWithMockData(getFakeAuthors(userIdcs), getFakePosts(userIdcs))
	if err != nil {
		return nil, nil, fmt.Errorf("seed profile repo: %w", err)
	}

	return userRepo, profileRepo, nil
}

func seedUsers(ctx context.Context, repo *authrepo.UserRepository) (map[string]uuid.UUID, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {

	}

	createdAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	idcs := make(map[string]uuid.UUID)
	for _, u := range []authmodels.User{
		{Username: "ivan001", Nickname: "Иван", Email: "ivan@gmail.com", PasswordHash: string(hash), Status: "active", CreatedAt: createdAt, UpdatedAt: createdAt},
		{Username: "alexandra", Nickname: "Александра", Email: "sasha@gmail.com", PasswordHash: string(hash), Status: "active", CreatedAt: createdAt, UpdatedAt: createdAt},
		{Username: "petr", Nickname: "Петр", Email: "petr@gmail.com", PasswordHash: string(hash), Status: "active", CreatedAt: createdAt, UpdatedAt: createdAt},
	} {
		createdUser, err := repo.CreateUser(ctx, &u)
		if err != nil {
			return nil, fmt.Errorf("seed user %s: %w", u.Username, err)
		}

		idcs[createdUser.Username] = createdUser.ID
	}

	return idcs, nil
}

func getFakeAuthors(userIdcs map[string]uuid.UUID) map[uuid.UUID]profilemodels.Author {
	return map[uuid.UUID]profilemodels.Author{
		userIdcs["ivan001"]:   {Bio: "Я автор Иван с двумя постами", Category: "Технологии"},
		userIdcs["alexandra"]: {Bio: "Я автор Александра без постов", Category: "Здоровье"},
	}
}

func getFakePosts(userIdcs map[string]uuid.UUID) []profilemodels.Post {
	ivanID := userIdcs["ivan001"]
	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	return []profilemodels.Post{
		{
			ID:          uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
			AuthorID:    ivanID,
			Title:       "Первый пост",
			Body:        "Привет, это мой первый пост и в нем я расскажу о чем нибудь.",
			Status:      profilemodels.PostStatusPublished,
			PublishedAt: toPtr(baseTime.Add(time.Hour)),
			CreatedAt:   baseTime,
		},
		{
			ID:          uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
			AuthorID:    ivanID,
			Title:       "Второй пост",
			Body:        "Это второй пост",
			Status:      profilemodels.PostStatusPublished,
			PublishedAt: toPtr(baseTime.Add(25 * time.Hour)),
			CreatedAt:   baseTime.Add(24 * time.Hour),
		},
	}
}

func toPtr[T any](v T) *T { return &v }

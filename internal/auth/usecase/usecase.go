package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Repository interface {
	CreateUser(ctx context.Context, user models.User) (models.User, error)
	GetUserByEmail(ctx context.Context, email string) (models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error)
}

type UserPayload struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"`
}

type TokenManager interface {
	Generate(payload UserPayload) (string, error)
}

type Usecase struct {
	repo           Repository
	tokenGenerator TokenManager
}

func NewUsecase(r Repository, t TokenManager) *Usecase {
	return &Usecase{repo: r, tokenGenerator: t}
}

func (uc *Usecase) Register(ctx context.Context, email, username, nickname, password string) (models.User, string, error) {
	if _, err := uc.repo.GetUserByEmail(ctx, email); err == nil {
		return models.User{}, "", models.ErrUserExists
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, "", fmt.Errorf("password hash error: %w", err)
	}

	user := models.User{
		ID:           uuid.New(),
		Email:        email,
		Username:     username,
		Nickname:     nickname,
		PasswordHash: string(passwordHash),
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	user, err = uc.repo.CreateUser(ctx, user)
	if err != nil {
		return models.User{}, "", err
	}

	token, err := uc.tokenGenerator.Generate(UserPayload{UserID: user.ID, Role: "user"})
	if err != nil {
		return models.User{}, "", err
	}

	return user, token, nil
}

func (uc *Usecase) Login(ctx context.Context, email, password string) (models.User, string, error) {
	user, err := uc.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return models.User{}, "", models.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return models.User{}, "", models.ErrInvalidCredentials
	}

	token, err := uc.tokenGenerator.Generate(UserPayload{UserID: user.ID, Role: "user"})
	if err != nil {
		return models.User{}, "", err
	}

	return user, token, nil
}

func (uc *Usecase) GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	return uc.repo.GetUserByID(ctx, id)
}

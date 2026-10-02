package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrRegistrationFailed = errors.New("registration failed")
	ErrLoginFailed        = errors.New("login failed")
	ErrInternal           = errors.New("internal server error")
	ErrUserIDNotFound     = errors.New("user with this userID was not found")
)

type Repository interface {
	CreateUser(ctx context.Context, user *models.User) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

type TokenGenerator interface {
	Generate(userID uuid.UUID) (string, error)
}

type Usecase struct {
	repo           Repository
	tokenGenerator TokenGenerator
}

func NewUsecase(r Repository, t TokenGenerator) *Usecase {
	return &Usecase{repo: r, tokenGenerator: t}
}

func (uc *Usecase) Register(ctx context.Context, email, username, nickname, password string) (*models.User, string, error) {
	email = normalizeEmail(email)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInternal, err)
	}

	user := &models.User{
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
		if errors.Is(err, models.ErrUserExists) {
			return nil, "", fmt.Errorf("%w: %w", ErrRegistrationFailed, err)
		}
		return nil, "", fmt.Errorf("%w: %w", ErrInternal, err)
	}

	token, err := uc.tokenGenerator.Generate(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInternal, err)
	}

	return user, token, nil
}

func (uc *Usecase) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	email = normalizeEmail(email)
	user, err := uc.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, "", fmt.Errorf("%w: %w", ErrLoginFailed, err)
		}
		return nil, "", fmt.Errorf("%w: %w", ErrInternal, err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrLoginFailed, err)
	}

	token, err := uc.tokenGenerator.Generate(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInternal, err)
	}

	return user, token, nil
}

func (uc *Usecase) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user, err := uc.repo.GetUserByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUserIDNotFound, err)
	}
	return user, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

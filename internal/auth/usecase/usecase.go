package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/token"
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
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByIdentifier(ctx context.Context, identifier string) (*models.User, error)
}

type TokenManager interface {
	Generate(payload any) (string, error)
	TTL() time.Duration
}

type Usecase struct {
	repo           Repository
	tokenGenerator TokenManager
}

func NewUsecase(r Repository, t TokenManager) *Usecase {
	return &Usecase{repo: r, tokenGenerator: t}
}

type RegisterInput struct {
	Email    string
	Username string
	Nickname string
	Password string
}

func (uc *Usecase) Register(ctx context.Context, regInput RegisterInput) (*models.User, string, error) {
	normalizedEmail := normalizeEmail(regInput.Email)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(regInput.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInternal, err)
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        normalizedEmail,
		Username:     regInput.Username,
		Nickname:     regInput.Nickname,
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

	token, err := uc.tokenGenerator.Generate(token.UserPayload{UserID: user.ID, Role: "user"})
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInternal, err)
	}

	return user, token, nil
}

func (uc *Usecase) Login(ctx context.Context, username, email, password string) (*models.User, string, error) {
	var identifier string
	if strings.TrimSpace(username) != "" {
		identifier = strings.TrimSpace(username)
	} else {
		identifier = normalizeEmail(email)
	}
	user, err := uc.repo.GetUserByIdentifier(ctx, identifier) // логиниться можно по email или username
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

	token, err := uc.tokenGenerator.Generate(token.UserPayload{UserID: user.ID, Role: "user"})
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

func (uc *Usecase) TokenTTL() time.Duration {
	return uc.tokenGenerator.TTL()
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

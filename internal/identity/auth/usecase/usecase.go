package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/dto"
	"github.com/google/uuid"
)

var (
	ErrRegistrationFailed = errors.New("registration failed")
	ErrLoginFailed        = errors.New("login failed")
	ErrInvalidToken       = errors.New("invalid token")
)

type UserUsecase interface {
	CreateUser(ctx context.Context, req userdto.CreateUserRequest) (*userdto.UserInfo, error)
	FindCredentialsByLogin(ctx context.Context, login string) (*userdto.UserCredentials, error)
}

type TokenManager interface {
	Generate(ID uuid.UUID, payload any) (string, error)
	Verify(token string) (json.RawMessage, error)
	TTL() time.Duration
}

type Usecase struct {
	users          UserUsecase
	tokenGenerator TokenManager
}

func NewUsecase(u UserUsecase, t TokenManager) *Usecase {
	return &Usecase{users: u, tokenGenerator: t}
}

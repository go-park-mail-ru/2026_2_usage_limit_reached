package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"golang.org/x/crypto/bcrypt"
)

type AdminProvisioner interface {
	EnsureAdmin(ctx context.Context, user models.User) error
}

func BootstrapAdmin(ctx context.Context, provisioner AdminProvisioner, email, username, password string) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap admin password: %w", err)
	}

	user := models.User{
		Email:        normalizeLogin(email),
		Username:     normalizeLogin(username),
		Nickname:     strings.TrimSpace(username),
		PasswordHash: string(passwordHash),
		Status:       models.StatusActive,
		Role:         models.RoleAdmin,
	}
	if err := provisioner.EnsureAdmin(ctx, user); err != nil {
		return fmt.Errorf("ensure bootstrap admin: %w", err)
	}

	return nil
}

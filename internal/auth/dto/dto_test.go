package dto_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/google/uuid"
)

func setupValidator() *validator.Validator {
	allowedRunes := make(map[rune]struct{})
	for _, r := range "abcdefghijklmnopqrstuvwxyz0123456789" {
		allowedRunes[r] = struct{}{}
	}

	return validator.NewValidator(&validator.ValidationConfig{
		MinUsernameLen:       3,
		MaxUsernameLen:       32,
		MaxNicknameLen:       64,
		MinPasswordLen:       8,
		MaxPasswordLen:       72,
		MaxEmailLen:          254,
		EmailRegexp:          regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`),
		UsernameAllowedRunes: allowedRunes,
	})
}

func TestRegistrationRequest_Validate(t *testing.T) {
	v := setupValidator()

	baseValid := dto.RegistrationRequest{
		Email:    "test@mail.ru",
		Username: "validuser",
		Nickname: "ValidNick",
		Password: "password123",
	}

	tests := []struct {
		name    string
		mutate  func(r *dto.RegistrationRequest)
		wantErr bool
	}{
		{
			name:    "All fields valid",
			mutate:  func(r *dto.RegistrationRequest) {},
			wantErr: false,
		},
		{
			name: "Invalid Email",
			mutate: func(r *dto.RegistrationRequest) {
				r.Email = "invalid-email"
			},
			wantErr: true,
		},
		{
			name: "Invalid Username",
			mutate: func(r *dto.RegistrationRequest) {
				r.Username = "ab"
			},
			wantErr: true,
		},
		{
			name: "Invalid Nickname",
			mutate: func(r *dto.RegistrationRequest) {
				r.Nickname = ""
			},
			wantErr: true,
		},
		{
			name: "Invalid Password",
			mutate: func(r *dto.RegistrationRequest) {
				r.Password = "123"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := baseValid
			tt.mutate(&req)

			err := req.Validate(v)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoginRequest_Validate(t *testing.T) {
	v := setupValidator()

	tests := []struct {
		name    string
		req     dto.LoginRequest
		wantErr bool
	}{
		{
			name: "Valid Login and Password",
			req: dto.LoginRequest{
				Login:    "user@mail.ru",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "Empty Login",
			req: dto.LoginRequest{
				Login:    "",
				Password: "password123",
			},
			wantErr: true,
		},
		{
			name: "Whitespace-only Login",
			req: dto.LoginRequest{
				Login:    "   ",
				Password: "password123",
			},
			wantErr: true,
		},
		{
			name: "Empty Password",
			req: dto.LoginRequest{
				Login:    "user@mail.ru",
				Password: "",
			},
			wantErr: true,
		},
		{
			name: "Whitespace-only Password",
			req: dto.LoginRequest{
				Login:    "user@mail.ru",
				Password: "   ",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate(v)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToUserInfo(t *testing.T) {
	t.Run("Nil User", func(t *testing.T) {
		res := dto.ToUserInfo(nil)
		if res != nil {
			t.Errorf("expected nil for nil user, got %v", res)
		}
	})

	t.Run("Valid User", func(t *testing.T) {
		now := time.Now()
		userID := uuid.New()

		u := &models.User{
			ID:        userID,
			Email:     "user@mail.ru",
			Username:  "username123",
			Nickname:  "NickName",
			AvatarKey: "avatars/1.png",
			CreatedAt: now,
		}

		res := dto.ToUserInfo(u)
		if res == nil {
			t.Fatal("expected non-nil UserInfo")
		}

		if res.ID != userID ||
			res.Email != u.Email ||
			res.Username != u.Username ||
			res.Nickname != u.Nickname ||
			res.AvatarKey != u.AvatarKey ||
			!res.CreatedAt.Equal(u.CreatedAt) {
			t.Errorf("ToUserInfo result does not match original user: %+v", res)
		}
	})
}

func TestToUserResponse(t *testing.T) {
	t.Run("Nil User", func(t *testing.T) {
		res := dto.ToUserResponse(nil)
		if res != nil {
			t.Errorf("expected nil for nil user, got %v", res)
		}
	})

	t.Run("Valid User", func(t *testing.T) {
		u := &models.User{
			Email:    "user@mail.ru",
			Username: "username123",
			Nickname: "NickName",
		}

		res := dto.ToUserResponse(u)
		if res == nil {
			t.Fatal("expected non-nil UserResponse")
		}

		if res.Email != u.Email ||
			res.Username != u.Username ||
			res.Nickname != u.Nickname {
			t.Errorf("ToUserResponse result does not match original user: %+v", res)
		}
	})
}
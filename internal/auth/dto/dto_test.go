package dto_test

import (
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	"github.com/stretchr/testify/require"
)

func TestRegistrationRequest_Validate(t *testing.T) {
	v := testutils.SetupValidator()

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
		{name: "All fields valid", mutate: func(r *dto.RegistrationRequest) {}, wantErr: false},
		{name: "Invalid Email", mutate: func(r *dto.RegistrationRequest) { r.Email = "invalid-email" }, wantErr: true},
		{name: "Invalid Username", mutate: func(r *dto.RegistrationRequest) { r.Username = "ab" }, wantErr: true},
		{name: "Invalid Nickname", mutate: func(r *dto.RegistrationRequest) { r.Nickname = "" }, wantErr: true},
		{name: "Invalid Password", mutate: func(r *dto.RegistrationRequest) { r.Password = "123" }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := baseValid
			tt.mutate(&req)

			err := req.Validate(v)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoginRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     dto.LoginRequest
		wantErr bool
	}{
		{name: "Valid Login and Password", req: dto.LoginRequest{Login: "user@mail.ru", Password: "password123"}, wantErr: false},
		{name: "Empty Login", req: dto.LoginRequest{Login: "", Password: "password123"}, wantErr: true},
		{name: "Whitespace-only Login", req: dto.LoginRequest{Login: "   ", Password: "password123"}, wantErr: true},
		{name: "Empty Password", req: dto.LoginRequest{Login: "user@mail.ru", Password: ""}, wantErr: true},
		{name: "Whitespace-only Password", req: dto.LoginRequest{Login: "user@mail.ru", Password: "   "}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestToUserResponse(t *testing.T) {
	avatar := "/media/avatars/ivan.png"

	t.Run("Nil user", func(t *testing.T) {
		require.Nil(t, dto.ToUserResponse(nil))
	})

	t.Run("Mapping", func(t *testing.T) {
		got := dto.ToUserResponse(&userdto.UserInfo{Username: "ivan001", Nickname: "Ivan", AvatarURL: &avatar, Email: "ivan@gmail.com"})

		require.Equal(t, &dto.UserResponse{Username: "ivan001", Nickname: "Ivan", AvatarURL: &avatar}, got)
	})
}

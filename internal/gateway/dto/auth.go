package dto

import "time"

type RegistrationRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Username string `json:"username" example:"username123"`
	Nickname string `json:"nickname" example:"nickname_123"`
	Password string `json:"password" example:"secret_password_123"`
}

type LoginRequest struct {
	Login    string `json:"login" example:"username123"`
	Password string `json:"password" example:"secret_password_123"`
}

type UserResponse struct {
	Username  string  `json:"username" example:"username123"`
	Nickname  string  `json:"nickname" example:"nickname_123"`
	AvatarURL *string `json:"avatar_url,omitempty" example:"/media/avatars/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000201.png"`
}

type AuthResult struct {
	User      UserResponse
	Token     string
	ExpiresIn time.Duration
}

type CSRFTokenResponse struct {
	CSRFToken string `json:"csrf_token" example:"q3VtX2Jhc2U2NHVybF9zYWx0X2FuZF9obWFjX3NpZ25hdHVyZV9leGFtcGxlXzAx"`
}

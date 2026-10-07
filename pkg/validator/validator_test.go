package validator_test

import (
	"strings"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/testutils"
	"github.com/stretchr/testify/require"
)

func TestValidator_IsValidEmail(t *testing.T) {
	v := testutils.SetupValidator()

	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{
			name:     "Valid email",
			email:    "test@mail.ru",
			expected: true,
		},
		{
			name:     "Email exceeds max length",
			email:    "verylongemailaddressthatfail@mail.ru",
			expected: false,
		},
		{
			name:     "Invalid regex email",
			email:    "invalid-email-format",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.IsValidEmail(tt.email)
			require.Equal(t, got, tt.expected)
		})
	}
}

func TestValidator_IsValidUsername(t *testing.T) {
	v := testutils.SetupValidator()

	tests := []struct {
		name     string
		username string
		expected bool
	}{
		{
			name:     "Valid username",
			username: "abc_12",
			expected: true,
		},
		{
			name:     "Empty or whitespace only",
			username: "   ",
			expected: false,
		},
		{
			name:     "Too short",
			username: "ab",
			expected: false,
		},
		{
			name:     "Too long",
			username: "abc_12abc_12abc_12abc_12abc_12abc_12abc_12abc_12",
			expected: false,
		},
		{
			name:     "Disallowed rune",
			username: "abc$12",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.IsValidUsername(tt.username)
			require.Equal(t, tt.expected, got, "IsValidUsername(%q) = %v, expected %v", tt.username, got, tt.expected)
		})
	}
}

func TestValidator_IsValidNickname(t *testing.T) {
	v := testutils.SetupValidator()

	tests := []struct {
		name     string
		nickname string
		expected bool
	}{
		{
			name:     "Valid nickname",
			nickname: "Nick_Name",
			expected: true,
		},
		{
			name:     "Empty nickname",
			nickname: "",
			expected: false,
		},
		{
			name:     "Too long nickname",
			nickname: strings.Repeat("n", 33),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.IsValidNickname(tt.nickname)
			require.Equal(t, tt.expected, got, "IsValidNickname(%q) = %v, expected %v", tt.nickname, got, tt.expected)
		})
	}
}

func TestValidator_IsValidPassword(t *testing.T) {
	v := testutils.SetupValidator()

	tests := []struct {
		name     string
		password string
		expected bool
	}{
		{
			name:     "Valid password",
			password: "secret_pass",
			expected: true,
		},
		{
			name:     "Too short",
			password: "12345",
			expected: false,
		},
		{
			name:     "Too long",
			password: "very_very_long_secret_password_123",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.IsValidPassword(tt.password)
			require.Equal(t, tt.expected, got, "IsValidPassword(%q) = %v, expected %v", tt.password, got, tt.expected)
		})
	}
}

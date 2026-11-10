package csrf_test

import (
	"strings"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/stretchr/testify/require"
)

const (
	testSecret  = "test-csrf-secret-with-at-least-32-symbols"
	otherSecret = "other-csrf-secret-with-at-least-32-symbols"
)

func newManager(t *testing.T, secret string, ttl time.Duration) *csrf.Manager {
	t.Helper()

	return csrf.NewManager(csrf.CSRFConfig{Secret: secret, ContextTTL: ttl})
}

func TestDefaultConfig(t *testing.T) {
	require.Equal(t, 24*time.Hour, csrf.DefaultConfig().ContextTTL)
}

func TestCSRFConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         csrf.CSRFConfig
		expectedErr []error
	}{
		{
			name: "Valid",
			cfg:  csrf.CSRFConfig{Secret: testSecret, ContextTTL: time.Hour},
		},
		{
			name:        "Short secret",
			cfg:         csrf.CSRFConfig{Secret: "short", ContextTTL: time.Hour},
			expectedErr: []error{csrf.ErrInvalidSecret},
		},
		{
			name:        "Zero ttl",
			cfg:         csrf.CSRFConfig{Secret: testSecret},
			expectedErr: []error{csrf.ErrInvalidContextTTL},
		},
		{
			name:        "Negative ttl",
			cfg:         csrf.CSRFConfig{Secret: testSecret, ContextTTL: -time.Second},
			expectedErr: []error{csrf.ErrInvalidContextTTL},
		},
		{
			name:        "Both invalid",
			cfg:         csrf.CSRFConfig{},
			expectedErr: []error{csrf.ErrInvalidSecret, csrf.ErrInvalidContextTTL},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()

			if len(tt.expectedErr) == 0 {
				require.NoError(t, err)
				return
			}
			for _, expected := range tt.expectedErr {
				require.ErrorIs(t, err, expected)
			}
		})
	}
}

func TestBindings(t *testing.T) {
	require.Equal(t, "auth:jwt", csrf.AuthBinding("jwt"))
	require.Equal(t, "anon:ctx", csrf.AnonBinding("ctx"))
}

func TestManager_ContextTTL(t *testing.T) {
	require.Equal(t, 3*time.Hour, newManager(t, testSecret, 3*time.Hour).ContextTTL())
}

func TestManager_NewContext(t *testing.T) {
	m := newManager(t, testSecret, time.Hour)

	first, err := m.NewContext()
	require.NoError(t, err)
	second, err := m.NewContext()
	require.NoError(t, err)

	require.NotEqual(t, first, second)
	require.True(t, m.IsValidContext(first))
	require.True(t, m.IsValidContext(second))
}

func TestManager_IsValidContext(t *testing.T) {
	m := newManager(t, testSecret, time.Hour)
	valid, err := m.NewContext()
	require.NoError(t, err)

	expired, err := newManager(t, testSecret, -time.Second).NewContext()
	require.NoError(t, err)

	foreign, err := newManager(t, otherSecret, time.Hour).NewContext()
	require.NoError(t, err)

	tampered := []byte(valid)
	tampered[5] ^= 1

	tests := []struct {
		name     string
		context  string
		expected bool
	}{
		{name: "Fresh context", context: valid, expected: true},
		{name: "Expired context", context: expired, expected: false},
		{name: "Signed with another secret", context: foreign, expected: false},
		{name: "Tampered context", context: string(tampered), expected: false},
		{name: "Truncated context", context: valid[:len(valid)-4], expected: false},
		{name: "Extended context", context: valid + "AAAA", expected: false},
		{name: "Not base64", context: "!!!not-base64!!!", expected: false},
		{name: "Old format random string", context: strings.Repeat("A", 43), expected: false},
		{name: "Empty", context: "", expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, m.IsValidContext(tt.context))
		})
	}
}

func TestManager_Binding(t *testing.T) {
	m := newManager(t, testSecret, time.Hour)
	anon, err := m.NewContext()
	require.NoError(t, err)

	expired, err := newManager(t, testSecret, -time.Second).NewContext()
	require.NoError(t, err)

	tests := []struct {
		name            string
		in              csrf.BindingInput
		expectedBinding string
		expectedOK      bool
	}{
		{
			name:            "Session token",
			in:              csrf.BindingInput{SessionToken: "jwt"},
			expectedBinding: "auth:jwt",
			expectedOK:      true,
		},
		{
			name:            "Session token has priority over anon context",
			in:              csrf.BindingInput{SessionToken: "jwt", AnonContext: anon},
			expectedBinding: "auth:jwt",
			expectedOK:      true,
		},
		{
			name:            "Valid anon context",
			in:              csrf.BindingInput{AnonContext: anon},
			expectedBinding: "anon:" + anon,
			expectedOK:      true,
		},
		{
			name: "Expired anon context",
			in:   csrf.BindingInput{AnonContext: expired},
		},
		{
			name: "Invalid anon context",
			in:   csrf.BindingInput{AnonContext: "garbage"},
		},
		{
			name: "Nothing",
			in:   csrf.BindingInput{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binding, ok := m.Binding(tt.in)

			require.Equal(t, tt.expectedOK, ok)
			require.Equal(t, tt.expectedBinding, binding)
		})
	}
}

func TestManager_Resolve(t *testing.T) {
	m := newManager(t, testSecret, time.Hour)
	anon, err := m.NewContext()
	require.NoError(t, err)

	expired, err := newManager(t, testSecret, -time.Second).NewContext()
	require.NoError(t, err)

	t.Run("Session token does not create context", func(t *testing.T) {
		binding, newContext, err := m.Resolve(csrf.BindingInput{SessionToken: "jwt"})

		require.NoError(t, err)
		require.Equal(t, "auth:jwt", binding)
		require.Empty(t, newContext)
	})

	t.Run("Valid anon context is reused", func(t *testing.T) {
		binding, newContext, err := m.Resolve(csrf.BindingInput{AnonContext: anon})

		require.NoError(t, err)
		require.Equal(t, "anon:"+anon, binding)
		require.Empty(t, newContext)
	})

	for name, in := range map[string]csrf.BindingInput{
		"Missing context is created":  {},
		"Expired context is replaced": {AnonContext: expired},
		"Invalid context is replaced": {AnonContext: "garbage"},
	} {
		t.Run(name, func(t *testing.T) {
			binding, newContext, err := m.Resolve(in)

			require.NoError(t, err)
			require.NotEmpty(t, newContext)
			require.NotEqual(t, in.AnonContext, newContext)
			require.True(t, m.IsValidContext(newContext))
			require.Equal(t, "anon:"+newContext, binding)
		})
	}
}

func TestManager_TokenVerify(t *testing.T) {
	m := newManager(t, testSecret, time.Hour)
	token, err := m.Token("auth:jwt")
	require.NoError(t, err)

	tampered := []byte(token)
	tampered[len(tampered)-1] ^= 1

	tests := []struct {
		name     string
		manager  *csrf.Manager
		binding  string
		token    string
		expected bool
	}{
		{name: "Valid", manager: m, binding: "auth:jwt", token: token, expected: true},
		{name: "Another binding", manager: m, binding: "auth:other", token: token},
		{name: "Anon binding with auth token", manager: m, binding: "anon:jwt", token: token},
		{name: "Another secret", manager: newManager(t, otherSecret, time.Hour), binding: "auth:jwt", token: token},
		{name: "Tampered token", manager: m, binding: "auth:jwt", token: string(tampered)},
		{name: "Truncated token", manager: m, binding: "auth:jwt", token: token[:len(token)-4]},
		{name: "Not base64", manager: m, binding: "auth:jwt", token: "!!!"},
		{name: "Empty token", manager: m, binding: "auth:jwt", token: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.manager.Verify(tt.binding, tt.token))
		})
	}
}

func TestManager_TokenIsRandomized(t *testing.T) {
	m := newManager(t, testSecret, time.Hour)

	first, err := m.Token("auth:jwt")
	require.NoError(t, err)
	second, err := m.Token("auth:jwt")
	require.NoError(t, err)

	require.NotEqual(t, first, second)
	require.True(t, m.Verify("auth:jwt", first))
	require.True(t, m.Verify("auth:jwt", second))
}

func TestManager_AnonTokenIsNotValidForSession(t *testing.T) {
	m := newManager(t, testSecret, time.Hour)
	anon, err := m.NewContext()
	require.NoError(t, err)

	binding, _, err := m.Resolve(csrf.BindingInput{AnonContext: anon})
	require.NoError(t, err)
	token, err := m.Token(binding)
	require.NoError(t, err)

	// Токен, выданный под анонимный контекст, не подходит авторизованному пользователю
	authBinding, ok := m.Binding(csrf.BindingInput{SessionToken: anon})
	require.True(t, ok)
	require.False(t, m.Verify(authBinding, token))
}

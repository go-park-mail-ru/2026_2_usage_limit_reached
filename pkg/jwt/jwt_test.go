package jwt_test

import (
	"strings"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const testSecret = "0123456789abcdef0123456789abcdef"

var testUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

func newTestManager(t *testing.T, secret string) *jwt.JWTManager {
	t.Helper()
	manager, err := jwt.NewJWTManager(&jwt.JWTConfig{Secret: secret, TokenTTL: time.Hour})
	require.NoError(t, err)
	return manager
}

func signedToken(t *testing.T, expiresAt int64) string {
	t.Helper()
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, jwtv5.MapClaims{
		"sub":     testUserID.String(),
		"exp":     expiresAt,
		"payload": jwt.UserPayload{UserID: testUserID, Role: "user"},
	})
	signed, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return signed
}

func TestNewJWTManager_Success(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ttl  time.Duration
		want time.Duration
	}{
		{name: "valid config", ttl: time.Hour, want: time.Hour},
		{name: "small positive TTL", ttl: time.Nanosecond, want: time.Nanosecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			manager, err := jwt.NewJWTManager(&jwt.JWTConfig{Secret: testSecret, TokenTTL: tt.ttl})
			require.NoError(t, err)
			require.Equal(t, tt.want, manager.TTL())
		})
	}
}

func TestNewJWTManager_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     jwt.JWTConfig
		wantErr error
	}{
		{name: "short secret", cfg: jwt.JWTConfig{Secret: testSecret[:31], TokenTTL: time.Hour}, wantErr: jwt.ErrInvalidSecretKey},
		{name: "empty secret", cfg: jwt.JWTConfig{TokenTTL: time.Hour}, wantErr: jwt.ErrInvalidSecretKey},
		{name: "zero TTL", cfg: jwt.JWTConfig{Secret: testSecret}, wantErr: jwt.ErrInvalidTTL},
		{name: "negative TTL", cfg: jwt.JWTConfig{Secret: testSecret, TokenTTL: -time.Second}, wantErr: jwt.ErrInvalidTTL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := jwt.NewJWTManager(&tt.cfg)
			require.Nil(t, manager)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestJWTManager_Generate_Success(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		payload any
		want    any
	}{
		{
			name:    "user payload",
			payload: jwt.UserPayload{UserID: testUserID, Role: "user"},
			want:    map[string]any{"user_id": testUserID.String(), "role": "user"},
		},
		{name: "nil payload", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token, err := newTestManager(t, testSecret).Generate(testUserID, tt.payload)
			require.NoError(t, err)

			claims := jwtv5.MapClaims{}
			parsed, err := jwtv5.ParseWithClaims(token, claims, func(*jwtv5.Token) (any, error) {
				return []byte(testSecret), nil
			}, jwtv5.WithValidMethods([]string{jwtv5.SigningMethodHS256.Alg()}))
			require.NoError(t, err)
			require.True(t, parsed.Valid)
			require.Equal(t, jwt.TokenTypeJWT, parsed.Header["typ"])
			require.Equal(t, testUserID.String(), claims["sub"])
			require.Equal(t, claims["iat"], claims["nbf"])
			require.Equal(t, float64(time.Hour/time.Second), claims["exp"].(float64)-claims["iat"].(float64))
			require.Equal(t, tt.want, claims["payload"])
		})
	}
}

func TestJWTManager_Generate_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		id      uuid.UUID
		payload any
		wantErr error
	}{
		{name: "nil user ID", id: uuid.Nil, wantErr: jwt.ErrInvalidUserID},
		{name: "unsupported payload", id: testUserID, payload: make(chan int)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token, err := newTestManager(t, testSecret).Generate(tt.id, tt.payload)
			require.Empty(t, token)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestJWTManager_GenerateVerify_RoundTrip(t *testing.T) {
	t.Parallel()
	manager := newTestManager(t, testSecret)
	payload := jwt.UserPayload{UserID: testUserID, Role: "user"}

	token, err := manager.Generate(testUserID, payload)
	require.NoError(t, err)

	got, err := manager.Verify(token)
	require.NoError(t, err)
	require.JSONEq(t, `{"user_id":"00000000-0000-4000-8000-000000000001","role":"user"}`, string(got))
}

func TestJWTManager_Verify_Success(t *testing.T) {
	t.Parallel()
	token := signedToken(t, 4102444800)
	got, err := newTestManager(t, testSecret).Verify(token)
	require.NoError(t, err)
	require.JSONEq(t, `{"user_id":"00000000-0000-4000-8000-000000000001","role":"user"}`, string(got))
}

func TestJWTManager_Verify_Errors(t *testing.T) {
	t.Parallel()
	token, err := newTestManager(t, testSecret).Generate(testUserID, jwt.UserPayload{UserID: testUserID, Role: "user"})
	require.NoError(t, err)
	parts := strings.Split(token, ".")

	tests := []struct {
		name    string
		token   string
		secret  string
		wantErr error
	}{
		{name: "different secret", token: token, secret: testSecret + "x", wantErr: jwt.ErrInvalidSign},
		{name: "changed header", token: "A" + parts[0] + "." + parts[1] + "." + parts[2], secret: testSecret, wantErr: jwt.ErrInvalidSign},
		{name: "changed claims", token: parts[0] + ".A" + parts[1] + "." + parts[2], secret: testSecret, wantErr: jwt.ErrInvalidSign},
		{name: "changed signature", token: token + "A", secret: testSecret, wantErr: jwt.ErrInvalidSign},
		{name: "invalid signature base64", token: parts[0] + "." + parts[1] + ".!", secret: testSecret, wantErr: jwt.ErrInvalidSign},
		{name: "expired token", token: signedToken(t, 1), secret: testSecret, wantErr: jwt.ErrTokenExpired},
		{name: "zero expiration", token: signedToken(t, 0), secret: testSecret, wantErr: jwt.ErrInvalidToken},
		{name: "malformed token", token: "invalid.token", secret: testSecret, wantErr: jwt.ErrInvalidToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := newTestManager(t, tt.secret).Verify(tt.token)
			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

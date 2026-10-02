package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidSecretKey = errors.New("secret key must be longer than 31")
	ErrInvalidTTL       = errors.New("TTL must be greater 0")
	ErrInvalidUserID    = errors.New("generate token is failed cause of nil user id")
	ErrInvalidToken     = errors.New("token does not match jwt")
	ErrInvalidSign      = errors.New("signature does not match")
	ErrTokenExpired     = errors.New("token expired")
	ErrTokenNotValidYet = errors.New("token is not valid yet")
)

const (
	AlgorithmHS256 = "HS256"
	TokenTypeJWT   = "JWT"
	CookieName     = "token"

	minSecretKeyLen = 32
)

type JWTConfig struct {
	Secret   string
	TokenTTL time.Duration
}

type jwtClaims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	NotBefore int64  `json:"nbf"`
	ExpiresAt int64  `json:"exp"`

	Role string `json:"role"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type JWTManager struct {
	secretKey []byte
	ttl       time.Duration
}

type UserPayload struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"`
}

func NewJWTManager(cfg *JWTConfig) (*JWTManager, error) {
	if len(cfg.Secret) < minSecretKeyLen {
		return nil, ErrInvalidSecretKey
	}
	if cfg.TokenTTL <= 0 {
		return nil, ErrInvalidTTL
	}
	return &JWTManager{secretKey: []byte(cfg.Secret), ttl: cfg.TokenTTL}, nil
}

func (m *JWTManager) Generate(payload any) (string, error) {
	userPayload, ok := payload.(UserPayload)
	if !ok {
		return "", ErrInvalidUserID // пока только для UserPayload может работать
	}
	if userPayload.UserID == uuid.Nil {
		return "", ErrInvalidUserID
	}

	now := time.Now()

	header := jwtHeader{
		Algorithm: AlgorithmHS256,
		Type:      TokenTypeJWT,
	}

	claims := jwtClaims{
		Subject:   userPayload.UserID.String(),
		Role:      userPayload.Role,
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		ExpiresAt: now.Add(m.ttl).Unix(),
	}

	encodedHeader, err := marshalAndEncodeToBase64(header)
	if err != nil {
		return "", err
	}

	encodedClaims, err := marshalAndEncodeToBase64(claims)
	if err != nil {
		return "", err
	}

	signingInput := encodedHeader + "." + encodedClaims
	sign := m.sign([]byte(signingInput))

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sign), nil
}

func (m *JWTManager) Verify(tokenStr string) (*UserPayload, error) {
	segments := strings.Split(tokenStr, ".")
	if len(segments) != 3 {
		return nil, ErrInvalidToken
	}

	encodedHeader, encodedClaims, encodedSign := segments[0], segments[1], segments[2]

	wantSign := m.sign([]byte(encodedHeader + "." + encodedClaims))
	gotSign, err := base64.RawURLEncoding.DecodeString(encodedSign)
	if err != nil || !hmac.Equal(wantSign, gotSign) {
		return nil, ErrInvalidSign
	}

	var claims jwtClaims
	if err := decodeAndUnmarshal(encodedClaims, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	now := time.Now().Unix()
	if claims.ExpiresAt == 0 {
		return nil, ErrInvalidToken
	}
	if now >= claims.ExpiresAt {
		return nil, ErrTokenExpired
	}
	if claims.NotBefore != 0 && now < claims.NotBefore {
		return nil, ErrTokenNotValidYet
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil || userID == uuid.Nil {
		return nil, ErrInvalidToken
	}

	return &UserPayload{UserID: userID, Role: claims.Role}, nil
}

func (m *JWTManager) TTL() time.Duration {
	return m.ttl
}

func (m *JWTManager) sign(data []byte) []byte {
	h := hmac.New(sha256.New, m.secretKey)
	h.Write(data)
	return h.Sum(nil)
}

func marshalAndEncodeToBase64(data any) (string, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(dataJSON), nil
}

func decodeAndUnmarshal[T any](data string, target *T) error {
	bytes, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, target)
}

package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTP       HTTPConfig
	JWT        JWTConfig
	CORS       CORSConfig
	Validation ValidationConfig
}

type HTTPConfig struct {
	Port    string
	Timeout time.Duration
}

type JWTConfig struct {
	Secret   string
	TokenTTL time.Duration
}

type CORSConfig struct {
	AllowedOrigins map[string]struct{}
}

type ValidationConfig struct {
	MinUsernameLen       int
	MaxUsernameLen       int
	MaxNicknameLen       int
	MinPasswordLen       int
	MaxPasswordLen       int
	MaxEmailLen          int
	EmailRegexp          *regexp.Regexp
	UsernameAllowedRunes map[rune]struct{}
}

const defaultUsernameAllowedRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()_+-="

func Load() (*Config, error) {
	HTTPcfg, err := loadHTTP()
	if err != nil {
		return nil, err
	}

	JWTcfg, err := loadJWT()
	if err != nil {
		return nil, err
	}

	CORScfg := loadCORS()

	ValidationCfg, err := loadValidation()
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTP:       HTTPcfg,
		JWT:        JWTcfg,
		CORS:       CORScfg,
		Validation: ValidationCfg,
	}, nil
}

func loadHTTP() (HTTPConfig, error) {
	timeoutStr := getEnv("HTTP_TIMEOUT", "5s")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return HTTPConfig{}, fmt.Errorf("invalid HTTP_TIMEOUT: %w", err)
	}

	port := ":" + getEnv("HTTP_PORT", "8080")

	return HTTPConfig{
		Port:    port,
		Timeout: timeout,
	}, nil
}

func loadJWT() (JWTConfig, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return JWTConfig{}, fmt.Errorf("JWT_SECRET is required")
	}

	ttlStr := getEnv("JWT_TOKEN_TTL", "24h")
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		return JWTConfig{}, fmt.Errorf("invalid JWT_TOKEN_TTL: %w", err)
	}

	return JWTConfig{
		Secret:   secret,
		TokenTTL: ttl,
	}, nil
}

func loadCORS() CORSConfig {
	raw := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	origins := make(map[string]struct{})
	for origin := range strings.SplitSeq(raw, ",") {
		origins[strings.TrimSpace(origin)] = struct{}{}
	}
	return CORSConfig{AllowedOrigins: origins}
}

func loadValidation() (ValidationConfig, error) {
	minUsernameLen, err := strconv.Atoi(getEnv("VAL_MINUSERNAMELEN", "3"))
	if err != nil {
		return ValidationConfig{}, fmt.Errorf("invalid VAL_MINUSERNAMELEN: %w", err)
	}

	maxUsernameLen, err := strconv.Atoi(getEnv("VAL_MAXUSERNAMELEN", "32"))
	if err != nil {
		return ValidationConfig{}, fmt.Errorf("invalid VAL_MAXUSERNAMELEN: %w", err)
	}

	maxNicknameLen, err := strconv.Atoi(getEnv("VAL_MAXNICKNAMELEN", "64"))
	if err != nil {
		return ValidationConfig{}, fmt.Errorf("invalid VAL_MAXNICKNAMELEN: %w", err)
	}

	minPasswordLen, err := strconv.Atoi(getEnv("VAL_MINPASSWORDLEN", "8"))
	if err != nil {
		return ValidationConfig{}, fmt.Errorf("invalid VAL_MINPASSWORDLEN: %w", err)
	}

	maxPasswordLen, err := strconv.Atoi(getEnv("VAL_MAXPASSWORDLEN", "72"))
	if err != nil {
		return ValidationConfig{}, fmt.Errorf("invalid VAL_MAXPASSWORDLEN: %w", err)
	}

	maxEmailLen, err := strconv.Atoi(getEnv("VAL_MAXEMAILLEN", "254"))
	if err != nil {
		return ValidationConfig{}, fmt.Errorf("invalid VAL_MAXEMAILLEN: %w", err)
	}

	if minUsernameLen > maxUsernameLen {
		return ValidationConfig{}, fmt.Errorf("VAL_MINUSERNAMELEN (%d) > VAL_MAXUSERNAMELEN (%d)", minUsernameLen, maxUsernameLen)
	}
	if minPasswordLen > maxPasswordLen {
		return ValidationConfig{}, fmt.Errorf("VAL_MINPASSWORDLEN (%d) > VAL_MAXPASSWORDLEN (%d)", minPasswordLen, maxPasswordLen)
	}

	emailRegexpStr := getEnv("VAL_EMAIL_REGEXP", `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	emailRegexp, err := regexp.Compile(emailRegexpStr)
	if err != nil {
		return ValidationConfig{}, fmt.Errorf("invalid VAL_EMAIL_REGEXP: %w", err)
	}

	allowedCharsStr := getEnv("VAL_USERNAME_ALLOWED_CHARS", defaultUsernameAllowedRunes)
	allowedRunes := make(map[rune]struct{}, len(allowedCharsStr))
	for _, r := range allowedCharsStr {
		allowedRunes[r] = struct{}{}
	}

	return ValidationConfig{
		MinUsernameLen:       minUsernameLen,
		MaxUsernameLen:       maxUsernameLen,
		MaxNicknameLen:       maxNicknameLen,
		MinPasswordLen:       minPasswordLen,
		MaxPasswordLen:       maxPasswordLen,
		MaxEmailLen:          maxEmailLen,
		EmailRegexp:          emailRegexp,
		UsernameAllowedRunes: allowedRunes,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

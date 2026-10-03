package config

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
)

type Config struct {
	HTTP       *httpserver.HTTPConfig
	JWT        *jwt.JWTConfig
	CORS       *middleware.CORSConfig
	Validation *validator.ValidationConfig
	Logger     *slog.Logger
}

const defaultUsernameAllowedRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_"

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

	validationCfg, err := loadValidation()
	if err != nil {
		return nil, err
	}

	logger := loadLogger()

	return &Config{
		HTTP:       HTTPcfg,
		JWT:        JWTcfg,
		CORS:       CORScfg,
		Validation: validationCfg,
		Logger:     logger,
	}, nil
}

func loadHTTP() (*httpserver.HTTPConfig, error) {
	timeoutStr := getEnv("HTTP_TIMEOUT", "5s")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP_TIMEOUT: %w", err)
	}

	shutdownTimeoutStr := getEnv("HTTP_SHUTDOWN", "25s")
	shutdownTimeout, err := time.ParseDuration(shutdownTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP_SHUTDOWN: %w", err)
	}

	port := ":" + getEnv("HTTP_PORT", "8080")

	return &httpserver.HTTPConfig{
		Port:            port,
		Timeout:         timeout,
		ShutdownTimeout: shutdownTimeout,
	}, nil
}

func loadJWT() (*jwt.JWTConfig, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	ttlStr := getEnv("JWT_TOKEN_TTL", "24h")
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_TOKEN_TTL: %w", err)
	}

	return &jwt.JWTConfig{
		Secret:   secret,
		TokenTTL: ttl,
	}, nil
}

func loadCORS() *middleware.CORSConfig {
	raw := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	origins := make(map[string]struct{})
	for origin := range strings.SplitSeq(raw, ",") {
		origins[strings.TrimSpace(origin)] = struct{}{}
	}
	return &middleware.CORSConfig{AllowedOrigins: origins}
}

func loadValidation() (*validator.ValidationConfig, error) {
	minUsernameLen, err := strconv.Atoi(getEnv("VAL_MINUSERNAMELEN", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid VAL_MINUSERNAMELEN: %w", err)
	}

	maxUsernameLen, err := strconv.Atoi(getEnv("VAL_MAXUSERNAMELEN", "32"))
	if err != nil {
		return nil, fmt.Errorf("invalid VAL_MAXUSERNAMELEN: %w", err)
	}

	maxNicknameLen, err := strconv.Atoi(getEnv("VAL_MAXNICKNAMELEN", "64"))
	if err != nil {
		return nil, fmt.Errorf("invalid VAL_MAXNICKNAMELEN: %w", err)
	}

	minPasswordLen, err := strconv.Atoi(getEnv("VAL_MINPASSWORDLEN", "8"))
	if err != nil {
		return nil, fmt.Errorf("invalid VAL_MINPASSWORDLEN: %w", err)
	}

	maxPasswordLen, err := strconv.Atoi(getEnv("VAL_MAXPASSWORDLEN", "72"))
	if err != nil {
		return nil, fmt.Errorf("invalid VAL_MAXPASSWORDLEN: %w", err)
	}

	maxEmailLen, err := strconv.Atoi(getEnv("VAL_MAXEMAILLEN", "254"))
	if err != nil {
		return nil, fmt.Errorf("invalid VAL_MAXEMAILLEN: %w", err)
	}

	if minUsernameLen > maxUsernameLen {
		return nil, fmt.Errorf("VAL_MINUSERNAMELEN (%d) > VAL_MAXUSERNAMELEN (%d)", minUsernameLen, maxUsernameLen)
	}
	if minPasswordLen > maxPasswordLen {
		return nil, fmt.Errorf("VAL_MINPASSWORDLEN (%d) > VAL_MAXPASSWORDLEN (%d)", minPasswordLen, maxPasswordLen)
	}

	emailRegexpStr := getEnv("VAL_EMAIL_REGEXP", `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	emailRegexp, err := regexp.Compile(emailRegexpStr)
	if err != nil {
		return nil, fmt.Errorf("invalid VAL_EMAIL_REGEXP: %w", err)
	}

	allowedCharsStr := getEnv("VAL_USERNAME_ALLOWED_CHARS", defaultUsernameAllowedRunes)
	allowedRunes := make(map[rune]struct{}, len(allowedCharsStr))
	for _, r := range allowedCharsStr {
		allowedRunes[r] = struct{}{}
	}

	return &validator.ValidationConfig{
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

func loadLogger() *slog.Logger {
	var baseHandler slog.Handler
	level := getEnv("LEVEL", "local")
	switch level {
	case "dev":
		baseHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	case "prod":
		baseHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	default:
		baseHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}

	handler := middleware.NewRequestIDHandler(baseHandler)
	return slog.New(handler)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

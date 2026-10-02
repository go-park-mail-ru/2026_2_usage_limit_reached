package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	AuthHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/token"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/repository/memory"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/gorilla/mux"
)

// @title Patreon Clone API — Auth
// @version 1.0
// @description Регистрация, вход и выход.
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name token
func main() {
	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tokens, err := token.NewJWTManager(cfg.JWT.Secret, cfg.JWT.TokenTTL)
	if err != nil {
		log.Fatalf("token manager initialization error: %v", err)
	}
	repo := memory.NewUserRepository()
	uc := usecase.NewUsecase(repo, tokens)
	valid := validator.NewValidator(
		cfg.Validation.MinUsernameLen,
		cfg.Validation.MaxUsernameLen,
		cfg.Validation.MaxNicknameLen,
		cfg.Validation.MinPasswordLen,
		cfg.Validation.MaxPasswordLen,
		cfg.Validation.MaxEmailLen,
		cfg.Validation.EmailRegexp,
		cfg.Validation.UsernameAllowedRunes,
	)
	authHandler := AuthHandlers.NewHandler(uc, cfg.JWT.TokenTTL, valid, logger)
	authMiddleware := middleware.AuthMiddleware(tokens)
	r := mux.NewRouter()
	private := r.NewRoute().Subrouter()
	private.Use(authMiddleware)
	authHandler.RegisterRoutes(r, private)

	handler := middleware.RecoverMiddleware(logger)(r)
	handler = middleware.AccessLogMiddleware(logger)(handler)
	handler = middleware.CORSMiddleware(cfg.CORS.AllowedOrigins)(handler)

	srv := &http.Server{
		Addr:              cfg.HTTP.Port,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTP.Timeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", slog.String("addr", cfg.HTTP.Port))
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server fatal error", slog.String("error", err.Error()))
			os.Exit(1)
		}
		return
	case <-appCtx.Done():
	}
	stop()

	logger.Info("shutting down server gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server fatal error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("server stopped gracefully")
}

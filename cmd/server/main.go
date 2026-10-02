package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	handlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/token"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/repository/memory"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/gorilla/mux"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tokens := token.NewManager(cfg.JWT.Secret, cfg.JWT.TokenTTL)
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
	h := handlers.NewHandler(uc, tokens.TTL(), valid, logger)

	authMiddleware := middleware.AuthMiddleware(tokens)
	r := mux.NewRouter()
	h.RegisterRoutes(r, authMiddleware)

	handler := middleware.CORSMiddleware(cfg.CORS.AllowedOrigins)(r)
	handler = middleware.AccessLogMiddleware(logger)(handler)
	handler = middleware.RecoverMiddleware(logger)(handler)

	srv := &http.Server{
		Addr:              cfg.HTTP.Port,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTP.Timeout,
	}
	logger.Info("server starting", slog.String("addr", cfg.HTTP.Port))
	log.Fatal(srv.ListenAndServe())
}

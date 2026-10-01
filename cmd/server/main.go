package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery/handlers"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/token"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/repository/memory"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/gorilla/mux"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tokens, err := token.NewJWTManager(cfg.JWTSecret, cfg.TokenTTL)
	if err != nil {
		log.Fatalf("token manager initialization error: %v", err)
	}
	repo := memory.NewUserRepository()
	uc := usecase.NewUsecase(repo, tokens)
	h := handlers.NewHandler(uc, cfg.TokenTTL, logger)

	authMiddleware := middleware.AuthMiddleware(tokens)
	r := mux.NewRouter()
	h.RegisterRoutes(r, authMiddleware)

	handler := middleware.CORSMiddleware(cfg.AllowedOrigin)(r)
	handler = middleware.AccessLogMiddleware(logger)(handler)
	handler = middleware.RecoverMiddleware(logger)(handler)

	srv := &http.Server{Addr: cfg.Addr, Handler: handler}
	logger.Info("server starting", slog.String("addr", cfg.Addr))
	log.Fatal(srv.ListenAndServe())
}

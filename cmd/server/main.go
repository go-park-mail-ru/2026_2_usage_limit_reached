package main

import (
	"log"

	AuthHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/repository/memory"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/token"
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	logger := cfg.Logger
	tokens, err := token.NewJWTManager(cfg.JWT)
	if err != nil {
		log.Fatalf("token manager initialization error: %v", err)
	}
	repo := memory.NewUserRepository()
	uc := usecase.NewUsecase(repo, tokens)
	valid := validator.NewValidator(cfg.Validation)
	authHandler := AuthHandlers.NewHandler(uc, valid, logger)
	authMiddleware := middleware.AuthMiddleware(tokens)
	r := mux.NewRouter()
	private := r.NewRoute().Subrouter()
	private.Use(authMiddleware)
	authHandler.RegisterRoutes(r, private)

	r.Use(
		middleware.RecoverMiddleware(logger),
		middleware.AccessLogMiddleware(logger),
		middleware.CORSMiddleware(cfg.CORS),
	)

	srv := httpserver.New(cfg.HTTP, logger)
	if err := srv.Run(r, authMiddleware); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

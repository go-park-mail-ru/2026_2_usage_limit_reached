package main

import (
	"flag"
	"log"

	AuthHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery"
	AuthRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/repository/user"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	PostHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/delivery"
	PostRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/repository"
	PostUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/usecase"
	ProfileHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/delivery"
	ProfileRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/repository/memory"
	ProfileUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/gorilla/mux"
)

// @title Patreon Clone API
// @version 1.0
// @description REST API для платформы публикации контента (аналог Patreon).
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name __Host-token
func main() {
	var configPath string
	flag.StringVar(&configPath, "config-file", config.DefaultConfigPath, "path to yaml config")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	appLogger := logger.New(cfg.Log)
	tokens, err := jwt.NewJWTManager(cfg.JWT)
	if err != nil {
		log.Fatalf("token manager initialization error: %v", err)
	}

	postgresClient, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		log.Fatalf("databse pool initialization error: %v", err)
	}
	defer postgresClient.Close()

	userRepo := AuthRepo.NewUserRepository(postgresClient, appLogger)
	profileRepo := ProfileRepo.NewProfileRepository()
	postRepo := PostRepo.NewPostRepository(postgresClient, appLogger)

	uc := usecase.NewUsecase(userRepo, tokens)
	valid, err := validator.NewValidator(cfg.Validation)
	if err != nil {
		log.Fatalf("validator initialization error: %v", err)
	}
	authHandler := AuthHandlers.NewHandler(uc, valid, appLogger)
	csrfManager := csrf.NewManager(cfg.CSRF)
	csrfHandler := AuthHandlers.NewCSRFHandler(csrfManager, appLogger)
	authMiddleware := middleware.AuthMiddleware(tokens)
	optionalAuthMiddleware := middleware.OptionalAuthMiddleware(tokens)
	csrfMiddleware := middleware.CSRFMiddleware(csrfManager)

	profileUc := ProfileUsecase.NewProfileUsecase(uc, profileRepo)
	profileHandler := ProfileHandlers.NewProfileHandler(profileUc, appLogger)

	postUc := PostUsecase.NewPostUsecase(postRepo)
	postHandler := PostHandlers.NewHandler(postUc, valid, appLogger)

	r := mux.NewRouter()
	r.Use(
		middleware.RecoverMiddleware(appLogger),
		middleware.AccessLogMiddleware(appLogger),
		middleware.CORSMiddleware(cfg.CORS),
	)

	public := r.NewRoute().Subrouter()
	public.Use(optionalAuthMiddleware, csrfMiddleware)

	private := r.NewRoute().Subrouter()
	private.Use(authMiddleware, csrfMiddleware)

	authHandler.RegisterRoutes(public, private)
	csrfHandler.RegisterRoutes(public)
	profileHandler.RegisterRoutes(public, private)
	postHandler.RegisterRoutes(public, private)

	srv, err := httpserver.New(cfg.HTTP, appLogger)
	if err != nil {
		log.Fatalf("http server initialization error: %v", err)
	}
	if err := srv.Run(r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

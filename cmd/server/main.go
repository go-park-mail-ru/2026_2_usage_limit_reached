package main

import (
	"flag"
	"log"

	AuthHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
	ProfileHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/delivery"
	ProfileRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/repository/memory"
	ProfileUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	UserRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/repository/user"
	UserUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/gorilla/mux"
)

// @title Patreon Clone API
// @version 1.0
// @description Регистрация, вход, выход и профиль пользователя.
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name token
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

	postgresCli, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		log.Fatalf("databse pool initialization error: %v", err)
	}
	defer postgresCli.Close()

	userRepo := UserRepo.NewUserRepository(postgresCli.Pool)
	profileRepo := ProfileRepo.NewProfileRepository()

	userUc := UserUsecase.NewUsecase(userRepo)
	uc := usecase.NewUsecase(userUc, tokens)
	valid, err := validator.NewValidator(cfg.Validation)
	if err != nil {
		log.Fatalf("validator initialization error: %v", err)
	}
	authHandler := AuthHandlers.NewHandler(uc, valid, appLogger)
	authMiddleware := middleware.AuthMiddleware(tokens)

	profileUc := ProfileUsecase.NewProfileUsecase(userUc, profileRepo)
	profileHandler := ProfileHandlers.NewProfileHandler(profileUc, appLogger)

	r := mux.NewRouter()
	private := r.NewRoute().Subrouter()
	private.Use(authMiddleware)
	authHandler.RegisterRoutes(r, private)
	profileHandler.RegisterRoutes(r, private)
	r.Use(
		middleware.RecoverMiddleware(appLogger),
		middleware.AccessLogMiddleware(appLogger),
		middleware.CORSMiddleware(cfg.CORS),
	)

	srv, err := httpserver.New(cfg.HTTP, appLogger)
	if err != nil {
		log.Fatalf("http server initialization error: %v", err)
	}
	if err := srv.Run(r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

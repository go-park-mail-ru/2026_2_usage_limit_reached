package main

import (
	"log"

	authhandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery"
	authusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/mockdata"
	profilehandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/delivery"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/gorilla/mux"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}
	tokens, err := jwt.NewJWTManager(cfg.JWT)
	if err != nil {
		log.Fatalf("token manager initialization error: %v", err)
	}

	users, profiles, err := mockdata.Load()
	if err != nil {
		log.Fatalf("mock data load error: %v", err)
	}
	usersUC := authusecase.NewUsecase(users, tokens)
	authHandler := authhandlers.NewHandler(usersUC, validator.NewValidator(cfg.Validation), cfg.Logger)
	profileHandler := profilehandlers.NewProfileHandler(profileusecase.NewProfileUsecase(usersUC, profiles), cfg.Logger)
	authMiddleware := middleware.AuthMiddleware(tokens)

	r := mux.NewRouter()
	private := r.NewRoute().Subrouter()
	private.Use(authMiddleware)
	authHandler.RegisterRoutes(r, private)
	profileHandler.RegisterRoutes(r, private)
	r.Use(
		middleware.RecoverMiddleware(cfg.Logger),
		middleware.AccessLogMiddleware(cfg.Logger),
		middleware.CORSMiddleware(cfg.CORS),
	)

	cfg.Logger.Info("mock data loaded")
	if err := httpserver.New(cfg.HTTP, cfg.Logger).Run(r, authMiddleware); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

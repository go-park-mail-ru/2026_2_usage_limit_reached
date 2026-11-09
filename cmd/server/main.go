package main

import (
	"flag"
	"log"

	AuthHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	AuthorRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/repository"
	AuthorUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
	DonationHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/delivery"
	DonationRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/repository"
	DonationUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/usecase"
	PaymentsRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/repository"
	PaymentsUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/usecase"
	PaymentProvider "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider/mock"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	PostHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/delivery"
	PostRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/repository"
	PostUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/usecase"
	ProfileHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/delivery"
	ProfileRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/repository/profile"
	ProfileUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	UserRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/repository/user"
	UserUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/s3"
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

	s3Client, err := s3.NewClient(cfg.S3)
	if err != nil {
		log.Fatalf("s3 client initialization error: %v", err)
	}

	userRepo := UserRepo.NewUserRepository(postgresClient)
	authorRepo := AuthorRepo.NewAuthorRepository(postgresClient)
	profileRepo := ProfileRepo.NewProfileRepository(postgresClient)
	postRepo := PostRepo.NewPostRepository(postgresClient, appLogger)
	paymentRepo := PaymentsRepo.NewPaymentRepository(postgresClient, appLogger)
	donationRepo := DonationRepo.NewDonationRepository(postgresClient, appLogger)

	userUc := UserUsecase.NewUsecase(userRepo, cfg.S3.PublicPath)
	uc := usecase.NewUsecase(userUc, tokens)
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

	authorUc := AuthorUsecase.NewUsecase(authorRepo)
	profileUc := ProfileUsecase.NewProfileUsecase(userUc, authorUc, profileRepo, s3Client, cfg.S3.PublicPath, appLogger)
	profileHandler := ProfileHandlers.NewProfileHandler(profileUc, valid, appLogger)

	postUc := PostUsecase.NewPostUsecase(postRepo, cfg.S3.PublicPath)
	postHandler := PostHandlers.NewHandler(postUc, valid, appLogger)

	provider := PaymentProvider.NewMockProvider()
	paymentUc := PaymentsUsecase.NewPaymentUsecase(paymentRepo, provider)

	donationUc := DonationUsecase.NewDonationUsecase(donationRepo, paymentUc)
	donationHandler := DonationHandlers.NewHandler(donationUc, valid, appLogger)

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
	donationHandler.RegisterRoutes(private)

	srv, err := httpserver.New(cfg.HTTP, appLogger)
	if err != nil {
		log.Fatalf("http server initialization error: %v", err)
	}
	if err := srv.Run(r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

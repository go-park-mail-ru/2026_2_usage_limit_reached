package app

import (
	"fmt"

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
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
	"github.com/gorilla/mux"
)

func Run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	appLogger := logger.New(cfg.Log)
	tokens, err := jwt.NewJWTManager(cfg.JWT)
	if err != nil {
		return fmt.Errorf("token manager initialization error: %w", err)
	}

	postgresClient, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		return fmt.Errorf("databse pool initialization error: %w", err)
	}
	defer postgresClient.Close()

	postRepo := PostRepo.NewPostRepository(postgresClient, appLogger)
	paymentRepo := PaymentsRepo.NewPaymentRepository(postgresClient, appLogger)
	donationRepo := DonationRepo.NewDonationRepository(postgresClient, appLogger)

	valid, err := validator.NewValidator(cfg.Validation)
	if err != nil {
		return fmt.Errorf("validator initialization error: %w", err)
	}
	csrfManager := csrf.NewManager(cfg.CSRF)
	authMiddleware := middleware.AuthMiddleware(tokens)
	optionalAuthMiddleware := middleware.OptionalAuthMiddleware(tokens)
	csrfMiddleware := middleware.CSRFMiddleware(csrfManager)

	postSanitizer, err := sanitizer.NewSanitizer(cfg.Sanitizer)
	if err != nil {
		return fmt.Errorf("sanitizer initialization error: %w", err)
	}

	postUc := PostUsecase.NewPostUsecase(postRepo, postSanitizer, valid, cfg.S3.PublicPath)
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

	postHandler.RegisterRoutes(public, private)
	donationHandler.RegisterRoutes(private)

	srv, err := httpserver.New(cfg.HTTP, appLogger)
	if err != nil {
		return fmt.Errorf("http server initialization error: %w", err)
	}
	if err := srv.Run(r); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

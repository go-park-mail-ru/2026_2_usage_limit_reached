package app

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	identityclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/identity"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
	DonationHandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/delivery"
	DonationRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/repository"
	DonationUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/usecase"
	PaymentsRepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/repository"
	PaymentsUsecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/usecase"
	PaymentProvider "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider/mock"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	appLogger := logger.New(cfg.Log)

	identityConn, err := grpc.NewClient(cfg.Identity.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("identity client initialization error: %w", err)
	}
	defer identityConn.Close()

	identityClient := identityclient.New(identityConn, cfg.Identity.Timeout, appLogger)

	postgresClient, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		return fmt.Errorf("databse pool initialization error: %w", err)
	}
	defer postgresClient.Close()

	paymentRepo := PaymentsRepo.NewPaymentRepository(postgresClient, appLogger)
	donationRepo := DonationRepo.NewDonationRepository(postgresClient, appLogger)

	valid, err := validator.NewValidator(cfg.Validation)
	if err != nil {
		return fmt.Errorf("validator initialization error: %w", err)
	}
	csrfManager := csrf.NewManager(cfg.CSRF)
	authMiddleware := middleware.AuthMiddleware(identityClient)
	optionalAuthMiddleware := middleware.OptionalAuthMiddleware(identityClient)
	csrfMiddleware := middleware.CSRFMiddleware(csrfManager)

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

	donationHandler.RegisterRoutes(private)

	srv, err := httpserver.New(cfg.HTTP, appLogger)
	if err != nil {
		return fmt.Errorf("http server initialization error: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return srv.Run(ctx, r)
}

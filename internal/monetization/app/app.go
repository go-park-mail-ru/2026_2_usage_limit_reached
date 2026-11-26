package app

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	identityclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/identity"
	monetizationconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/config"
	monetizationhttp "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/delivery/http"
	httprouter "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/delivery/http/router"
	monetizationrepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/repository"
	monetizationuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase"
	expirationworker "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_expiration_worker"
	paymentprovider "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider/mock"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Run(configPath string) error {
	cfg, err := monetizationconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	appLogger := logger.New(cfg.Log)

	postgresClient, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		return fmt.Errorf("database pool initialization error: %w", err)
	}
	defer postgresClient.Close()

	valid, err := validator.NewValidator(cfg.Validation)
	if err != nil {
		return fmt.Errorf("validator initialization error: %w", err)
	}

	donationRepo := monetizationrepo.NewMonetizationRepository(postgresClient, appLogger)

	identityConn, err := grpc.NewClient(cfg.Identity.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("identity client initialization error: %w", err)
	}
	defer identityConn.Close()
	identityClient := identityclient.New(identityConn, cfg.Identity.Timeout, appLogger)

	monetizationUc := monetizationuc.New(
		donationRepo,
		donationRepo,
		paymentprovider.NewMockProvider(),
		identityClient,
		valid,
	)

	csrfManager := csrf.NewManager(cfg.CSRF)

	router := mux.NewRouter()
	router.Use(
		middleware.RecoverMiddleware(appLogger),
		middleware.AccessLogMiddleware(appLogger),
		middleware.CORSMiddleware(cfg.CORS),
	)

	private := router.NewRoute().Subrouter()
	private.Use(middleware.AuthMiddleware(identityClient), middleware.CSRFMiddleware(csrfManager))

	httprouter.RegisterRoutes(private, httprouter.Handlers{
		Donation: monetizationhttp.NewDonationHandler(monetizationUc, valid, appLogger),
	})

	httpSrv, err := httpserver.New(cfg.Monetization.HTTP, appLogger)
	if err != nil {
		return fmt.Errorf("http server initialization error: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return httpSrv.Run(gctx, router)
	})

	g.Go(func() error {
		expirationworker.RunExpirationWorker(gctx, donationRepo, cfg.Monetization.ExpirationWorker, appLogger)
		return nil
	})

	return g.Wait()
}

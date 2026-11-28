package app

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	identityclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/identity"
	onlineconfclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/onlineconf"
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

	monetizationRepo := monetizationrepo.NewMonetizationRepository(postgresClient, appLogger)

	identityConn, err := grpc.NewClient(cfg.Identity.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("identity client initialization error: %w", err)
	}
	defer identityConn.Close()
	identityClient := identityclient.New(identityConn, cfg.Identity.Timeout, appLogger)

	onlineconfCfg := cfg.Onlineconf
	if err := onlineconfCfg.Validate(); err != nil {
		return fmt.Errorf("onlineconf client config error: %w", err)
	}
	onlineconfConn, err := grpc.NewClient(onlineconfCfg.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("onlineconf client initialization error: %w", err)
	}
	defer onlineconfConn.Close()
	onlineconfClient := onlineconfclient.New(onlineconfConn, onlineconfCfg, appLogger)

	monetizationUc := monetizationuc.New(
		monetizationRepo,
		monetizationRepo,
		paymentprovider.NewMockProvider(),
		identityClient,
		onlineconfClient,
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

	routerDeps := httprouter.Router{
		Donation: monetizationhttp.NewDonationHandler(monetizationUc, valid, appLogger),
	}
	httprouter.RegisterPublicRoutes(router, routerDeps)
	httprouter.RegisterRoutes(private, routerDeps)

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
		expirationworker.RunExpirationWorker(gctx, monetizationRepo, cfg.Monetization.ExpirationWorker, appLogger)
		return nil
	})

	g.Go(func() error {
		onlineconfClient.Run(gctx)
		return nil
	})

	return g.Wait()
}

package app

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	identityclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/identity"
	paymentconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/config"
	paymentgrpc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/delivery/grpc"
	paymentrepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/repository"
	paymentuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/usecase"
	expirationworker "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_expiration_worker"
	paymentprovider "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider/mock"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/grpcserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/interceptors"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	paymentpb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/payment/v1"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Run(configPath string) error {
	cfg, err := paymentconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	appLogger := logger.New(cfg.Log)

	postgresClient, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		return fmt.Errorf("database pool initialization error: %w", err)
	}
	defer postgresClient.Close()

	paymentRepo := paymentrepo.NewPaymentRepository(postgresClient, appLogger)

	identityConn, err := grpc.NewClient(cfg.Identity.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("identity client initialization error: %w", err)
	}
	defer identityConn.Close()
	identityClient := identityclient.New(identityConn, cfg.Identity.Timeout)

	provider := paymentprovider.NewMockProvider()

	paymentUc := paymentuc.NewPaymentUsecase(paymentRepo, provider)

	valid, err := validator.NewValidator(cfg.Validation)
	if err != nil {
		return fmt.Errorf("validator initialization error: %w", err)
	}

	paymentHandler := paymentgrpc.NewHandler(paymentUc, valid, appLogger)
	grpcApp := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.UnaryRecovery(appLogger),
			interceptors.UnaryLogging(appLogger),
		),
	)
	paymentpb.RegisterPaymentServiceServer(grpcApp, paymentHandler)

	grpcSrv, err := grpcserver.New(cfg.Payment.GRPC, appLogger, grpcApp)
	if err != nil {
		return fmt.Errorf("grpc server initialization error: %w", err)
	}

	csrfManager := csrf.NewManager(cfg.CSRF)

	router := mux.NewRouter()
	router.Use(
		middleware.RecoverMiddleware(appLogger),
		middleware.AccessLogMiddleware(appLogger),
		middleware.CORSMiddleware(cfg.CORS),
	)

	public := router.NewRoute().Subrouter()
	public.Use(middleware.OptionalAuthMiddleware(identityClient), middleware.CSRFMiddleware(csrfManager))

	private := router.NewRoute().Subrouter()
	private.Use(middleware.AuthMiddleware(identityClient), middleware.CSRFMiddleware(csrfManager))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return grpcSrv.Run(gctx)
	})

	g.Go(func() error {
		expirationworker.RunExpirationWorker(gctx, paymentRepo, cfg.Payment.ExpirationWorker, appLogger)
		return nil
	})

	return g.Wait()
}

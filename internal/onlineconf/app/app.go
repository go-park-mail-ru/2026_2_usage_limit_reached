package app

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	identityclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/identity"
	onlineconfconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/config"
	onlineconfgrpc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/delivery/grpc"
	onlineconfhttp "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/delivery/http"
	httprouter "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/delivery/http/router"
	onlineconfrepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/repository"
	onlineconfuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/grpcserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/interceptors"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	onlineconfpb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/onlineconf/v1"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Run(configPath string) error {
	cfg, err := onlineconfconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	appLogger := logger.New(cfg.Log)

	postgresClient, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		return fmt.Errorf("database pool initialization error: %w", err)
	}
	defer postgresClient.Close()

	repo := onlineconfrepo.NewRepository(postgresClient, appLogger)
	uc := onlineconfuc.New(repo, appLogger)

	grpcApp := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.UnaryRecovery(appLogger),
			interceptors.UnaryLogging(appLogger),
		),
	)
	onlineconfpb.RegisterOnlineConfServiceServer(grpcApp, onlineconfgrpc.NewHandler(uc, appLogger))

	grpcSrv, err := grpcserver.New(cfg.Onlineconf.GRPC, appLogger, grpcApp)
	if err != nil {
		return fmt.Errorf("grpc server initialization error: %w", err)
	}

	identityConn, err := grpc.NewClient(cfg.Identity.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("identity client initialization error: %w", err)
	}
	defer identityConn.Close()

	identityClient := identityclient.New(identityConn, cfg.Identity.Timeout, appLogger)
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
	private.Use(
		middleware.NoStoreMiddleware,
		middleware.AuthMiddleware(identityClient),
		middleware.CSRFMiddleware(csrfManager),
		middleware.PermissionMiddleware(uc, appLogger),
	)

	httprouter.RegisterRoutes(public, private, httprouter.Handlers{
		Access:  onlineconfhttp.NewAccessHandler(appLogger),
		Scopes:  onlineconfhttp.NewScopesHandler(uc, appLogger),
		Configs: onlineconfhttp.NewConfigsHandler(uc, appLogger),
		Flags:   onlineconfhttp.NewFlagsHandler(uc, appLogger),
		History: onlineconfhttp.NewHistoryHandler(uc, appLogger),
		CSRF:    onlineconfhttp.NewCSRFHandler(csrfManager, appLogger),
	})

	httpSrv, err := httpserver.New(cfg.Onlineconf.HTTP, appLogger)
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
		return grpcSrv.Run(gctx)
	})

	return g.Wait()
}

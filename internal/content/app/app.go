package app

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	identityclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/identity"
	contentconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/config"
	contenthttp "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/delivery/http"
	httprouter "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/delivery/http/router"
	contentrepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/repository"
	contentuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Run(configPath string) error {
	cfg, err := contentconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	appLogger := logger.New(cfg.Log)

	valid, err := validator.NewValidator(cfg.Validation)
	if err != nil {
		return fmt.Errorf("validator initialization error: %w", err)
	}

	postgresClient, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		return fmt.Errorf("database pool initialization error: %w", err)
	}
	defer postgresClient.Close()

	postRepo := contentrepo.NewRepository(postgresClient, appLogger)
	postSanitizer, err := sanitizer.NewSanitizer(cfg.Sanitizer)
	if err != nil {
		return fmt.Errorf("sanitizer initialization error: %w", err)
	}

	identityConn, err := grpc.NewClient(cfg.Identity.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("identity client initialization error: %w", err)
	}
	defer identityConn.Close()

	identityClient := identityclient.New(identityConn, cfg.Identity.Timeout, appLogger)
	contentUc := contentuc.New(postRepo, identityClient, postSanitizer, valid, cfg.S3.PublicPath, appLogger)

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

	httprouter.RegisterRoutes(public, private, httprouter.Handlers{
		Post: contenthttp.NewPostHandler(contentUc, valid, appLogger),
	})

	httpSrv, err := httpserver.New(cfg.Content.HTTP, appLogger)
	if err != nil {
		return fmt.Errorf("http content server initialization error: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return httpSrv.Run(gctx, router)
	})

	return g.Wait()
}

package app

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	authuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/usecase"
	authorrepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/repository"
	authoruc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/usecase"
	identityconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/config"
	identitygrpc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/grpc"
	identityhttp "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/http"
	httprouter "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/http/router"
	profilerepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/repository/profile"
	profileuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/usecase"
	userrepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/repository/user"
	useruc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/grpcserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/interceptors"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/s3"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func Run(configPath string) error {
	cfg, err := identityconfig.Load(configPath)
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
		return fmt.Errorf("database pool initialization error: %w", err)
	}
	defer postgresClient.Close()

	s3Client, err := s3.NewClient(cfg.S3)
	if err != nil {
		return fmt.Errorf("s3 client initialization error: %w", err)
	}

	userRepo := userrepo.NewUserRepository(postgresClient, appLogger)
	authorRepo := authorrepo.NewAuthorRepository(postgresClient, appLogger)
	profileRepo := profilerepo.NewProfileRepository(postgresClient, appLogger)

	userUc := useruc.NewUsecase(userRepo, cfg.S3.PublicPath)
	authUc := authuc.NewUsecase(userUc, tokens)
	authorUc := authoruc.NewUsecase(authorRepo)
	profileUc := profileuc.NewProfileUsecase(userUc, authorUc, profileRepo, s3Client, cfg.S3.PublicPath, appLogger)

	valid, err := validator.NewValidator(cfg.Validation)
	if err != nil {
		return fmt.Errorf("validator initialization error: %w", err)
	}

	identityHandler := identitygrpc.NewHandler(userUc, authorUc, tokens, appLogger)
	grpcApp := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.UnaryRecovery(appLogger),
			interceptors.UnaryLogging(appLogger),
		),
	)
	identitypb.RegisterIdentityServiceServer(grpcApp, identityHandler)

	grpcSrv, err := grpcserver.New(cfg.Identity.GRPC, appLogger, grpcApp)
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
	public.Use(middleware.OptionalAuthMiddleware(tokens), middleware.CSRFMiddleware(csrfManager))

	private := router.NewRoute().Subrouter()
	private.Use(middleware.AuthMiddleware(tokens), middleware.CSRFMiddleware(csrfManager))

	httprouter.RegisterRoutes(public, private, httprouter.Handlers{
		Auth:    identityhttp.NewAuthHandler(authUc, valid, appLogger),
		Profile: identityhttp.NewProfileHandler(profileUc, valid, appLogger),
		CSRF:    identityhttp.NewCSRFHandler(csrfManager, appLogger),
	})

	httpSrv, err := httpserver.New(cfg.Identity.HTTP, appLogger)
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

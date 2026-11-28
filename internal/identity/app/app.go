package app

import (
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	identityconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/config"
	identitygrpc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/grpc"
	identityhttp "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/http"
	httprouter "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/http/router"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	identityrepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/repository"
	identityuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/usecase"
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

	repo := identityrepo.NewRepository(postgresClient, appLogger)

	valid, err := validator.NewValidator(cfg.Validation)
	if err != nil {
		return fmt.Errorf("validator initialization error: %w", err)
	}

	identityUc := identityuc.New(repo, tokens, s3Client, cfg.S3.PublicPath, appLogger)
	adminInput := dto.RegistrationRequest{
		Email:    strings.TrimSpace(cfg.Admin.Email),
		Username: strings.TrimSpace(cfg.Admin.Username),
		Nickname: strings.TrimSpace(cfg.Admin.Username),
		Password: cfg.Admin.Password,
	}
	if err := adminInput.Validate(valid); err != nil {
		return fmt.Errorf("bootstrap admin config validation error: %w", err)
	}
	bootstrapCtx, cancelBootstrap := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelBootstrap()
	if err := identityuc.BootstrapAdmin(bootstrapCtx, repo, adminInput.Email, adminInput.Username, adminInput.Password); err != nil {
		return fmt.Errorf("admin bootstrap error: %w", err)
	}

	grpcApp := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.UnaryRecovery(appLogger),
			interceptors.UnaryLogging(appLogger),
		),
	)
	identitypb.RegisterIdentityServiceServer(grpcApp, identitygrpc.NewHandler(identityUc, appLogger))

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
	public.Use(middleware.OptionalAuthMiddleware(identityUc), middleware.CSRFMiddleware(csrfManager))

	private := router.NewRoute().Subrouter()
	private.Use(middleware.AuthMiddleware(identityUc), middleware.CSRFMiddleware(csrfManager))

	httprouter.RegisterRoutes(public, private, httprouter.Router{
		Auth:    identityhttp.NewAuthHandler(identityUc, valid, appLogger),
		Profile: identityhttp.NewProfileHandler(identityUc, valid, appLogger),
		CSRF:    identityhttp.NewCSRFHandler(csrfManager, appLogger),
		Author:  identityhttp.NewAuthorHandler(identityUc, valid, appLogger),
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

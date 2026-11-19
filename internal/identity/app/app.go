package app

import (
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
	authuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/usecase"
	authorrepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/repository"
	authoruc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/usecase"
	identitygrpc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/grpc"
	profilerepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/repository/profile"
	profileuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/usecase"
	userrepo "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/repository/user"
	useruc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/grpcserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/interceptors"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/s3"
	"google.golang.org/grpc"
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

	handler := identitygrpc.NewHandler(authUc, profileUc, valid, appLogger)

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptors.UnaryRecovery(appLogger), interceptors.UnaryLogging(appLogger), identitygrpc.UserContextInterceptor), grpc.ChainStreamInterceptor(interceptors.StreamRecovery(appLogger), interceptors.StreamLogging(appLogger), identitygrpc.UserContextStreamInterceptor))
	identitypb.RegisterIdentityServiceServer(grpcServer, handler)

	server, err := grpcserver.New(cfg.GRPC, appLogger, grpcServer)
	if err != nil {
		return fmt.Errorf("grpc server initialization error: %w", err)
	}

	return server.Run()
}

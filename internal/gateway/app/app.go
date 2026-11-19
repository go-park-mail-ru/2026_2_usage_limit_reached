package app

import (
	"fmt"
	"net/http"

	identityclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/clients/identity"
	gatewayconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/config"
	gatewayhandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/delivery"
	gatewayrouter "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/delivery/router"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Run(configPath string) error {
	cfg, err := gatewayconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	appLogger := logger.New(cfg.Log)

	tokens, err := jwt.NewJWTManager(cfg.JWT)
	if err != nil {
		return fmt.Errorf("token manager initialization error: %w", err)
	}

	identityConn, err := grpc.NewClient(cfg.Gateway.Identity.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("identity client initialization error: %w", err)
	}
	defer identityConn.Close()

	identityClient := identityclient.New(identityConn, cfg.Gateway.Identity.Timeout)

	authMiddleware := middleware.AuthMiddleware(tokens)
	optionalAuthMiddleware := middleware.OptionalAuthMiddleware(tokens)

	csrfManager := csrf.NewManager(cfg.CSRF)
	csrfMiddleware := middleware.CSRFMiddleware(csrfManager)

	authHandler := gatewayhandlers.NewAuthHandler(identityClient, appLogger)
	profileHandler := gatewayhandlers.NewProfileHandler(identityClient, appLogger)
	csrfHandler := gatewayhandlers.NewCSRFHandler(csrfManager, appLogger)

	r := mux.NewRouter()
	r.Use(
		stripUserIDHeader,
		middleware.RecoverMiddleware(appLogger),
		middleware.AccessLogMiddleware(appLogger),
		middleware.CORSMiddleware(cfg.CORS),
	)

	public := r.NewRoute().Subrouter()
	public.Use(optionalAuthMiddleware, csrfMiddleware)

	private := r.NewRoute().Subrouter()
	private.Use(authMiddleware, csrfMiddleware)

	gatewayrouter.RegisterRoutes(public, private, gatewayrouter.Tools{
		Auth:    authHandler,
		Profile: profileHandler,
		CSRF:    csrfHandler,
	})

	srv, err := httpserver.New(cfg.HTTP, appLogger)
	if err != nil {
		return fmt.Errorf("http server initialization error: %w", err)
	}
	if err := srv.Run(r); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func stripUserIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("X-User-Id")
		next.ServeHTTP(w, r)
	})
}

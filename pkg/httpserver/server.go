package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
)

type HTTPConfig struct {
	Port              string        `mapstructure:"port"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
}

func DefaultConfig() HTTPConfig {
	return HTTPConfig{
		Port:              "8080",
		ReadHeaderTimeout: 5 * time.Second,
		ShutdownTimeout:   5 * time.Second,
	}
}

func (c HTTPConfig) Validate() error {
	var errs []error
	if port, err := strconv.Atoi(c.Port); err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Errorf("httpserver: port must be a number from 1 to 65535, got %q", c.Port))
	}
	if c.ReadHeaderTimeout <= 0 {
		errs = append(errs, fmt.Errorf("httpserver: read_header_timeout must be positive, got %s", c.ReadHeaderTimeout))
	}
	if c.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("httpserver: shutdown_timeout must be positive, got %s", c.ShutdownTimeout))
	}
	return errors.Join(errs...)
}

type Server struct {
	port              string
	readHeaderTimeout time.Duration
	shutdownTimeout   time.Duration
	logger            *slog.Logger
}

func New(cfg HTTPConfig, logger *slog.Logger) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Server{
		port:              ":" + cfg.Port,
		readHeaderTimeout: cfg.ReadHeaderTimeout,
		shutdownTimeout:   cfg.ShutdownTimeout,
		logger:            logger,
	}, nil
}

func (s *Server) Run(ctx context.Context, r *mux.Router) error {
	srv := &http.Server{
		Addr:              s.port,
		Handler:           r,
		ReadHeaderTimeout: s.readHeaderTimeout,
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		s.logger.Info("server starting", slog.String("addr", s.port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("server fatal error", slog.String("error", err.Error()))
			return err
		}

		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		s.logger.Info("shutting down server gracefully")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("server forced to shutdown", slog.String("error", err.Error()))
			return err
		}

		s.logger.Info("server stopped gracefully")
		return nil
	})

	return g.Wait()
}

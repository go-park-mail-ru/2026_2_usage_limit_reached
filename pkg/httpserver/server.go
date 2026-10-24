package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
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

func (s *Server) Run(r *mux.Router) error {
	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              s.port,
		Handler:           r,
		ReadHeaderTimeout: s.readHeaderTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		s.logger.Info("server starting", slog.String("addr", s.port))
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("server fatal error", slog.String("error", err.Error()))
			return err
		}
		return nil
	case <-appCtx.Done():
	}
	stop()

	s.logger.Info("shutting down server gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("server forced to shutdown", slog.String("error", err.Error()))
		return err
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("server fatal error", slog.String("error", err.Error()))
		return err
	}

	s.logger.Info("server stopped gracefully")
	return nil
}

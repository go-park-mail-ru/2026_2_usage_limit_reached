package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
)

type GRPCConfig struct {
	Port            string        `mapstructure:"port"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

func DefaultConfig() GRPCConfig {
	return GRPCConfig{
		Port:            "50051",
		ShutdownTimeout: 5 * time.Second,
	}
}

func (c GRPCConfig) Validate() error {
	var errs []error

	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Errorf("grpcserver: port must be a number from 1 to 65535, got %q", c.Port))
	}
	if c.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("grpcserver: shutdown_timeout must be positive, got %s", c.ShutdownTimeout))
	}

	return errors.Join(errs...)
}

type Server struct {
	port            string
	shutdownTimeout time.Duration
	logger          *slog.Logger
	grpc            *grpc.Server
}

func New(cfg GRPCConfig, logger *slog.Logger, server *grpc.Server) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if server == nil {
		return nil, errors.New("grpcserver: grpc server must not be nil")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Server{
		port:            cfg.Port,
		shutdownTimeout: cfg.ShutdownTimeout,
		logger:          logger,
		grpc:            server,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	addr := ":" + s.port

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("grpcserver: listen %s: %w", addr, err)
	}

	serveErr := make(chan error, 1)
	go func() {
		s.logger.Info("server starting", slog.String("addr", listener.Addr().String()))
		if err := s.grpc.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			s.logger.Error("server fatal error", slog.String("error", err.Error()))
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	s.logger.Info("shutting down server gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		s.grpc.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-shutdownCtx.Done():
		s.grpc.Stop()
		s.logger.Error("server forced to shutdown", slog.String("error", shutdownCtx.Err().Error()))
		return shutdownCtx.Err()
	}

	if err := <-serveErr; err != nil {
		s.logger.Error("server fatal error", slog.String("error", err.Error()))
		return err
	}

	s.logger.Info("server stopped gracefully")
	return nil
}

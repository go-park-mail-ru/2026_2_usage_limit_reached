package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

type Server struct {
	port            string
	readTimeout     time.Duration
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

func New(port string, readTimeout, shutdownTimeout time.Duration, logger *slog.Logger) *Server {
	return &Server{
		port:            port,
		readTimeout:     readTimeout,
		shutdownTimeout: shutdownTimeout,
		logger:          logger,
	}
}

func (s *Server) Run(r *mux.Router, authMiddleware mux.MiddlewareFunc) error {
	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              s.port,
		Handler:           r,
		ReadHeaderTimeout: s.readTimeout,
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

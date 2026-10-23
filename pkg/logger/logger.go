package logger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

type LoggerConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

var levels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func DefaultConfig() LoggerConfig {
	return LoggerConfig{Level: "info", Format: "json"}
}

func (c LoggerConfig) Validate() error {
	var errs []error
	if _, ok := levels[c.Level]; !ok {
		errs = append(errs, fmt.Errorf("logger: level must be one of debug, info, warn, error, got %q", c.Level))
	}
	if c.Format != "text" && c.Format != "json" {
		errs = append(errs, fmt.Errorf("logger: format must be text or json, got %q", c.Format))
	}
	return errors.Join(errs...)
}

func New(cfg LoggerConfig) *slog.Logger {
	opts := &slog.HandlerOptions{Level: levels[cfg.Level]}

	var baseHandler slog.Handler
	switch cfg.Format {
	case "text":
		baseHandler = slog.NewTextHandler(os.Stdout, opts)
	default:
		baseHandler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(NewRequestIDHandler(baseHandler))
}

type RequestIDKey struct{}

type RequestIDHandler struct {
	slog.Handler
}

func NewRequestIDHandler(h slog.Handler) *RequestIDHandler {
	return &RequestIDHandler{Handler: h}
}

func (h *RequestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID, ok := ctx.Value(RequestIDKey{}).(string); ok {
		r.AddAttrs(slog.String("request_id", requestID))
	}

	return h.Handler.Handle(ctx, r)
}

func (h *RequestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &RequestIDHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *RequestIDHandler) WithGroup(name string) slog.Handler {
	return &RequestIDHandler{Handler: h.Handler.WithGroup(name)}
}

package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
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

func AccessLogMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := uuid.New().String()
			ctx := context.WithValue(r.Context(), RequestIDKey{}, requestID)
			r = r.WithContext(ctx)

			log.InfoContext(ctx, "HTTP start",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
			)

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			duration := time.Since(start)

			level := slog.LevelInfo
			if rec.status >= 500 {
				level = slog.LevelError
			} else if rec.status >= 400 {
				level = slog.LevelWarn
			}

			log.LogAttrs(ctx, level, "HTTP finish",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.String("duration", duration.String()),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
			)
		})
	}
}

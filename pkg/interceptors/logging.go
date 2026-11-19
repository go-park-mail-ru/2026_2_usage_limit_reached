package interceptors

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func UnaryLogging(logger *slog.Logger) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = slog.Default()
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		attrs := []any{slog.String("method", info.FullMethod), slog.String("code", status.Code(err).String()), slog.Duration("duration", time.Since(start))}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
		}
		logger.InfoContext(ctx, "grpc request", attrs...)
		return resp, err
	}
}

func StreamLogging(logger *slog.Logger) grpc.StreamServerInterceptor {
	if logger == nil {
		logger = slog.Default()
	}

	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, stream)
		attrs := []any{slog.String("method", info.FullMethod), slog.String("code", status.Code(err).String()), slog.Duration("duration", time.Since(start))}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
		}
		logger.InfoContext(stream.Context(), "grpc stream", attrs...)
		return err
	}
}

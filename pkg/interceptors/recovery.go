package interceptors

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UnaryRecovery(logger *slog.Logger) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = slog.Default()
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(ctx, "grpc panic recovered", slog.String("method", info.FullMethod), slog.Any("panic", recovered))
				resp = nil
				err = status.Error(codes.Internal, "internal error")
			}
		}()

		return handler(ctx, req)
	}
}

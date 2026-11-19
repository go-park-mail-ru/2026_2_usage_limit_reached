package grpc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type userIDKey struct{}

func UserContextInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return handler(ctx, req)
	}

	values := md.Get("x-user-id")
	if len(values) == 0 {
		return handler(ctx, req)
	}

	userID, err := uuid.Parse(values[0])
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid x-user-id")
	}

	ctx = context.WithValue(ctx, userIDKey{}, userID)
	return handler(ctx, req)
}

func UserContextStreamInterceptor(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return handler(srv, stream)
	}

	values := md.Get("x-user-id")
	if len(values) == 0 {
		return handler(srv, stream)
	}

	userID, err := uuid.Parse(values[0])
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid x-user-id")
	}

	ctx = context.WithValue(ctx, userIDKey{}, userID)
	return handler(srv, &contextServerStream{ServerStream: stream, ctx: ctx})
}

type contextServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context {
	return s.ctx
}

func userIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	return userID, nil
}

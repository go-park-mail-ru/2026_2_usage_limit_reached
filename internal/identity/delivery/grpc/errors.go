package grpc

import (
	"context"
	"errors"
	"log/slog"

	authuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/usecase"
	authoruc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/usecase"
	profileuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) fail(ctx context.Context, action string, err error) error {
	h.logError(ctx, action, err)
	return mapError(err)
}

func (h *Handler) logError(ctx context.Context, action string, err error) {
	if h.logger == nil {
		return
	}

	level := slog.LevelError
	if isExpectedError(err) {
		level = slog.LevelInfo
	}

	h.logger.Log(ctx, level, "identity error", slog.String("handler", action), slog.String("error", err.Error()))
}

func isExpectedError(err error) bool {
	return errors.Is(err, validator.ErrValidation) ||
		errors.Is(err, authuc.ErrRegistrationFailed) ||
		errors.Is(err, authuc.ErrLoginFailed) ||
		errors.Is(err, authoruc.ErrAuthorNotFound) ||
		errors.Is(err, profileuc.ErrProfileNotFound) ||
		errors.Is(err, profileuc.ErrAvatarTooLarge) ||
		errors.Is(err, profileuc.ErrUnsupportedAvatarType) ||
		errors.Is(err, profileuc.ErrNotAuthor) ||
		errors.Is(err, profileuc.ErrUserInactive)
}

func mapError(err error) error {
	switch {
	case errors.Is(err, validator.ErrValidation):
		return status.Error(codes.InvalidArgument, "validation failed")
	case errors.Is(err, authuc.ErrRegistrationFailed):
		return status.Error(codes.AlreadyExists, "user already exists")
	case errors.Is(err, authuc.ErrLoginFailed):
		return status.Error(codes.Unauthenticated, "invalid credentials")
	case errors.Is(err, authoruc.ErrAuthorNotFound):
		return status.Error(codes.NotFound, "author not found")
	case errors.Is(err, profileuc.ErrProfileNotFound):
		return status.Error(codes.NotFound, "profile not found")
	case errors.Is(err, profileuc.ErrAvatarTooLarge):
		return status.Error(codes.ResourceExhausted, "avatar too large")
	case errors.Is(err, profileuc.ErrUnsupportedAvatarType):
		return status.Error(codes.InvalidArgument, "unsupported avatar type")
	case errors.Is(err, profileuc.ErrNotAuthor):
		return status.Error(codes.PermissionDenied, "not an author")
	case errors.Is(err, profileuc.ErrUserInactive):
		return status.Error(codes.PermissionDenied, "user is inactive")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

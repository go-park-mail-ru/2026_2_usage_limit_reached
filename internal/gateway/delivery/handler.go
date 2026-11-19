package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/apperrors"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
)

type Identity interface {
	Register(ctx context.Context, req dto.RegistrationRequest) (*dto.AuthResult, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResult, error)
	GetMe(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error)
	GetPublicProfile(ctx context.Context, username string) (*dto.PublicProfileResponse, error)
	UpdateMyProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) (*dto.ProfileResponse, error)
	BecomeAuthor(ctx context.Context, userID uuid.UUID, req dto.UpdateAuthorRequest) (*dto.ProfileResponse, error)
	UploadAvatar(ctx context.Context, userID uuid.UUID, file io.Reader, contentType string) (string, error)
	DeleteAvatar(ctx context.Context, userID uuid.UUID) error
}

func handleError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, action string, err error) {
	logError(ctx, logger, action, err)

	switch {
	case errors.Is(err, apperrors.ErrBadRequest):
		response.ErrorBadRequest(w)
	case errors.Is(err, apperrors.ErrUnauthorized):
		response.ErrorUnauthorized(w)
	case errors.Is(err, apperrors.ErrForbidden):
		response.ErrorForbidden(w)
	case errors.Is(err, apperrors.ErrNotFound):
		response.ErrorNotFound(w)
	case errors.Is(err, apperrors.ErrConflict):
		response.ErrorConflict(w)
	case errors.Is(err, apperrors.ErrPayloadTooLarge):
		response.ErrorPayloadTooLarge(w)
	case errors.Is(err, apperrors.ErrUnsupportedMediaType):
		response.ErrorUnsupportedMediaType(w)
	case errors.Is(err, apperrors.ErrServiceUnavailable):
		response.WriteJSON(w, http.StatusServiceUnavailable, response.ErrorResponse{Error: "service unavailable"})
	default:
		response.ErrorInternal(w)
	}
}

func logError(ctx context.Context, logger *slog.Logger, action string, err error) {
	if logger == nil {
		return
	}

	logger.ErrorContext(ctx, "gateway error",
		slog.String("handler", action),
		slog.String("error", err.Error()),
	)
}

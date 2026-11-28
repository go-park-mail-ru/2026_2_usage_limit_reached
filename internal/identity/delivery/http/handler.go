package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	identityuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
)

type IdentityUsecase interface {
	Register(ctx context.Context, req dto.RegistrationRequest) (*dto.UserResponse, string, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.UserResponse, string, error)
	TokenTTL() time.Duration

	GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error)
	GetPublicProfile(ctx context.Context, username string) (*dto.PublicProfileResponse, error)
	UpdateMyProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) (*dto.ProfileResponse, error)
	UploadAvatar(ctx context.Context, userID uuid.UUID, file io.ReadSeeker, size int64) (string, error)
	DeleteAvatar(ctx context.Context, userID uuid.UUID) error
}

func handleError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, action string, err error) {
	logError(ctx, logger, action, err)

	switch {
	case errors.Is(err, validator.ErrValidation), errors.Is(err, identityuc.ErrInvalidNickname):
		response.ErrorBadRequest(w)
	case errors.Is(err, identityuc.ErrRegistrationFailed):
		response.ErrorConflict(w)
	case errors.Is(err, identityuc.ErrLoginFailed):
		response.ErrorUnauthorized(w)
	case errors.Is(err, identityuc.ErrUserInactive):
		response.ErrorForbidden(w)
	case errors.Is(err, identityuc.ErrProfileNotFound), errors.Is(err, identityuc.ErrUserNotFound), errors.Is(err, identityuc.ErrAuthorNotFound):
		response.ErrorNotFound(w)
	case errors.Is(err, identityuc.ErrAvatarTooLarge):
		response.ErrorPayloadTooLarge(w)
	case errors.Is(err, identityuc.ErrUnsupportedAvatarType):
		response.ErrorUnsupportedMediaType(w)
	default:
		response.ErrorInternal(w)
	}
}

func logError(ctx context.Context, logger *slog.Logger, action string, err error) {
	if logger == nil {
		return
	}

	logger.ErrorContext(ctx, "identity http error",
		slog.String("handler", action),
		slog.String("error", err.Error()),
	)
}

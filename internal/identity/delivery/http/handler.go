package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	authdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/dto"
	authuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/usecase"
	profiledto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/dto"
	profileuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
)

type AuthUsecase interface {
	Register(ctx context.Context, req authdto.RegistrationRequest) (*authdto.UserResponse, string, error)
	Login(ctx context.Context, req authdto.LoginRequest) (*authdto.UserResponse, string, error)
	TokenTTL() time.Duration
}

type ProfileUsecase interface {
	GetMyProfile(ctx context.Context, userID uuid.UUID) (*profiledto.ProfileResponse, error)
	GetPublicProfile(ctx context.Context, username string) (*profiledto.PublicProfileResponse, error)
	UpdateMyProfile(ctx context.Context, userID uuid.UUID, req profiledto.UpdateProfileRequest) (*profiledto.ProfileResponse, error)
	BecomeAuthor(ctx context.Context, userID uuid.UUID, req profiledto.UpdateAuthorRequest) (*profiledto.ProfileResponse, error)
	UploadAvatar(ctx context.Context, userID uuid.UUID, file io.ReadSeeker, size int64) (string, error)
	DeleteAvatar(ctx context.Context, userID uuid.UUID) error
}

func handleError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, action string, err error) {
	logError(ctx, logger, action, err)

	switch {
	case errors.Is(err, validator.ErrValidation):
		response.ErrorBadRequest(w)
	case errors.Is(err, authuc.ErrRegistrationFailed):
		response.ErrorConflict(w)
	case errors.Is(err, authuc.ErrLoginFailed):
		response.ErrorUnauthorized(w)
	case errors.Is(err, profileuc.ErrUserInactive), errors.Is(err, profileuc.ErrNotAuthor):
		response.ErrorForbidden(w)
	case errors.Is(err, profileuc.ErrProfileNotFound):
		response.ErrorNotFound(w)
	case errors.Is(err, profileuc.ErrAvatarTooLarge):
		response.ErrorPayloadTooLarge(w)
	case errors.Is(err, profileuc.ErrUnsupportedAvatarType):
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

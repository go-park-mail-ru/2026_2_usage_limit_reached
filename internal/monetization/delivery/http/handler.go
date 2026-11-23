package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	donationusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type DonationUsecase interface {
	Donate(ctx context.Context, payerID uuid.UUID, donationReq dto.CreateDonationRequest) (*dto.SentDonation, error)
	GetSentDonations(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error)
	GetReceivedDonations(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error)
}

func handleError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, action string, err error) {
	logError(ctx, logger, action, err)

	switch {
	case errors.Is(err, donationusecase.ErrForbidden):
		response.ErrorForbidden(w)
	case errors.Is(err, donationusecase.ErrNotFound):
		response.ErrorNotFound(w)
	case errors.Is(err, validator.ErrValidation), errors.Is(err, donationusecase.ErrValidation):
		response.ErrorBadRequest(w)
	case errors.Is(err, donationusecase.ErrConflict):
		response.ErrorConflict(w)
	default:
		response.ErrorInternal(w)
	}
}

func logError(ctx context.Context, logger *slog.Logger, action string, err error) {
	if logger == nil {
		return
	}

	logger.ErrorContext(ctx, "monetization http error",
		slog.String("handler", action),
		slog.String("error", err.Error()),
	)
}

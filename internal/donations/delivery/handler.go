package donationhandlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	donationusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type PaymentUsecase interface {
	Donate(ctx context.Context, payerID uuid.UUID, donationReq dto.CreateDonationRequest) (*dto.SentDonation, error)
	GetSentDonations(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error)
	GetReceivedDonations(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error)
}

type DonationHandler struct {
	uc        PaymentUsecase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewHandler(uc PaymentUsecase, v *validator.Validator, logger *slog.Logger) *DonationHandler {
	return &DonationHandler{uc: uc, validator: v, logger: logger}
}

func (h *DonationHandler) RegisterRoutes(private *mux.Router) {
	private.HandleFunc("/authors/{author_id}/donations", h.SendDonation).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/profile/me/donations/sent", h.GetMySentDonations).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me/donations/received", h.GetMyReceivedDonations).Methods(http.MethodGet, http.MethodOptions)
}

func (h *DonationHandler) handleError(ctx context.Context, w http.ResponseWriter, handler string, logMessage string, err error) {
	h.logger.ErrorContext(ctx, logMessage,
		slog.String("handler", handler),
		slog.String("error", err.Error()),
	)
	switch {
	case errors.Is(err, donationusecase.ErrForbidden):
		response.ErrorForbidden(w)
	case errors.Is(err, donationusecase.ErrNotFound):
		response.ErrorNotFound(w)
	case errors.Is(err, validator.ErrValidation), errors.Is(err, donationusecase.ErrValidation):
		response.ErrorBadRequest(w)
	default:
		response.ErrorInternal(w)
	}
}

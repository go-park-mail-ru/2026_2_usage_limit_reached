package donationhandlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/cursor"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	limitField   = "limit"
	cursorFiled  = "cursor"
	defaultLimit = 20
	maxLimit     = 50
)

type getDonationsFunc func(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) (*dto.DonationPage, error)

func (h *DonationHandler) SendDonation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var donationReq dto.CreateDonationRequest
	authorID, err := uuid.Parse(mux.Vars(r)["author_id"])
	if err != nil {
		h.handleError(ctx, w, "send donation", "invalid author id", fmt.Errorf("invalid author id: %w: %w", validator.ErrValidation, err))
		return
	}
	donationReq.AuthorID = authorID
	if err := response.DecodeJSON(w, r, &donationReq); err != nil {
		h.handleError(ctx, w, "send donation", "request body decode error", fmt.Errorf("invalid request body: %w: %w", validator.ErrValidation, err))
		return
	}

	if err := donationReq.Validate(h.validator); err != nil {
		h.handleError(ctx, w, "send donation", "validation error", err)
		return
	}

	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	donation, err := h.uc.Donate(ctx, user.UserID, donationReq)
	if err != nil {
		h.handleError(ctx, w, "send donation", "donate error", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, dto.CreateDonationResponse{PaymentOrderID: donation.PaymentOrderID, Status: donation.Status})
}

func (h *DonationHandler) GetMySentDonations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	resp, err := h.getMyDonations(ctx, query, user.UserID, h.uc.GetSentDonations)
	if err != nil {
		h.handleError(ctx, w, "get my sent donations", "", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *DonationHandler) GetMyReceivedDonations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	resp, err := h.getMyDonations(ctx, query, user.UserID, h.uc.GetReceivedDonations)
	if err != nil {
		h.handleError(ctx, w, "get my received donations", "", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *DonationHandler) getMyDonations(ctx context.Context, query url.Values, userID uuid.UUID, getDonations getDonationsFunc) (*dto.DonationPageResponse, error) {
	limit := feed.ParseLimit(query.Get(limitField), defaultLimit, maxLimit)
	rawCursor := query.Get(cursorFiled)
	cur, err := DonationsCursor.Decode(rawCursor)
	if err != nil {
		return nil, fmt.Errorf("decode cursor error: %w: %w", validator.ErrValidation, err)
	}

	page, err := getDonations(ctx, userID, cur, limit+1)
	if err != nil {
		return nil, err
	}
	next, err := DonationsCursor.Encode(page.Next)
	if err != nil {
		if errors.Is(err, cursor.ErrInvalidCursor) {
			return nil, fmt.Errorf("encode next cursor error: %w: %w", validator.ErrValidation, err)
		}
		return nil, fmt.Errorf("encode next cursor error: %w", err)
	}

	return &dto.DonationPageResponse{Items: page.Items, NextCursor: next}, nil
}

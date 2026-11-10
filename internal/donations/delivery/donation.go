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

// SendDonation отправляет донат автору
// @Summary Отправка доната
// @Description Отправляет донат указанному автору. Операция создаёт платёжную попытку, заказ и возвращает ID и статус заказа.
// @Tags donations
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param author_id path string true "ID автора" Format(uuid)
// @Param request body dto.CreateDonationRequest true "Данные доната"
// @Success 200 {object} dto.CreateDonationResponse "Заказ создан"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации или некорректный ID автора"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 404 {object} response.ErrorResponse "Автор не найден"
// @Failure 409 {object} response.ErrorResponse "Конфликт оплаты"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /authors/{author_id}/donations [post]
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

// GetMySentDonations возвращает список донатов, отправленных текущим пользователем
// @Summary Мои отправленные донаты
// @Description Возвращает список донатов, отправленных авторизованным пользователем, страницами. Чтобы получить следующую страницу, передайте next_cursor из ответа в параметр cursor; значение null означает, что страниц больше нет.
// @Tags donations
// @Produce json
// @Security CookieAuth
// @Param limit query int false "Размер страницы (значения вне диапазона 1-50 заменяются на 20)" default(20) minimum(1) maximum(50)
// @Param cursor query string false "Курсор из next_cursor предыдущего ответа (для первой страницы не передаётся)"
// @Success 200 {object} dto.DonationPageResponse "Страница донатов"
// @Failure 400 {object} response.ErrorResponse "Неверный cursor"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/donations/sent [get]
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

// GetMyReceivedDonations возвращает список донатов, полученных текущим пользователем
// @Summary Мои полученные донаты
// @Description Возвращает список донатов, полученных авторизованным пользователем (если он является автором), страницами. Чтобы получить следующую страницу, передайте next_cursor из ответа в параметр cursor; значение null означает, что страниц больше нет.
// @Tags donations
// @Produce json
// @Security CookieAuth
// @Param limit query int false "Размер страницы (значения вне диапазона 1-50 заменяются на 20)" default(20) minimum(1) maximum(50)
// @Param cursor query string false "Курсор из next_cursor предыдущего ответа (для первой страницы не передаётся)"
// @Success 200 {object} dto.DonationPageResponse "Страница донатов"
// @Failure 400 {object} response.ErrorResponse "Неверный cursor"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/donations/received [get]
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

	page, err := getDonations(ctx, userID, cur, limit)
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

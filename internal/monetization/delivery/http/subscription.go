package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/cursor"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type SubscriptionHandler struct {
	uc        MonetizationUsecase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewSubscriptionHandler(uc MonetizationUsecase, v *validator.Validator, logger *slog.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{uc: uc, validator: v, logger: logger}
}

// GetMySubscriptionTiers returns all tiers owned by the authenticated author.
// @Summary Список уровней подписки автора
// @Description Возвращает открытые и архивные уровни текущего автора страницами по возрастанию цены и ID. Для следующей страницы передайте next_cursor из ответа в параметр cursor.
// @Tags subscriptions
// @Produce json
// @Security CookieAuth
// @Param limit query int false "Размер страницы (значения вне диапазона 1-50 заменяются на 20)" default(20) minimum(1) maximum(50)
// @Param cursor query string false "Курсор из next_cursor предыдущего ответа (для первой страницы не передаётся)"
// @Success 200 {object} dto.SubscriptionTierListResponse
// @Failure 400 {object} response.ErrorResponse "Неверный cursor"
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /profile/me/tiers [get]
func (h *SubscriptionHandler) GetMySubscriptionTiers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	page, next, err := h.getSubscriptionTiers(ctx, user.ID, user.ID, nil, r.URL.Query(), true)
	if err != nil {
		handleError(ctx, h.logger, w, "get my subscription tiers", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, dto.SubscriptionTierListResponse{
		Items:      page.Items,
		NextCursor: next,
	})
}

// CreateSubscriptionTier creates a new open tier for the authenticated author.
// @Summary Создание уровня подписки
// @Description Создаёт открытый уровень подписки с фиксированной ценой за 30 суток.
// @Tags subscriptions
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param request body dto.CreateSubscriptionTierRequest true "Данные уровня"
// @Success 201 {object} dto.SubscriptionTier
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /profile/me/tiers [post]
func (h *SubscriptionHandler) CreateSubscriptionTier(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.CreateSubscriptionTierRequest
	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(ctx, h.logger, w, "create subscription tier",
			fmt.Errorf("decode tier request: %w: %w", validator.ErrValidation, err))
		return
	}
	req = req.Normalize()
	if err := req.Validate(h.validator); err != nil {
		handleError(ctx, h.logger, w, "create subscription tier", err)
		return
	}

	tier, err := h.uc.CreateSubscriptionTier(ctx, user.ID, req)
	if err != nil {
		handleError(ctx, h.logger, w, "create subscription tier", err)
		return
	}
	if tier == nil {
		logError(ctx, h.logger, "create subscription tier", errors.New("usecase returned nil tier"))
		response.ErrorInternal(w)
		return
	}

	response.WriteJSON(w, http.StatusCreated, tier)
}

// UpdateSubscriptionTier updates tier details or archives the tier.
// @Summary Изменение или архивация уровня подписки
// @Description Изменяет имя, описание и цену уровня либо архивирует его. Цена активного уровня не может пересечь цену соседнего уровня.
// @Tags subscriptions
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path string true "ID уровня" Format(uuid)
// @Param request body dto.UpdateSubscriptionTierRequest true "Изменения уровня"
// @Success 200 {object} dto.SubscriptionTier
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /profile/me/tiers/{id} [patch]
func (h *SubscriptionHandler) UpdateSubscriptionTier(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	tierID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		handleError(ctx, h.logger, w, "update subscription tier",
			fmt.Errorf("parse tier id: %w: %w", validator.ErrValidation, err))
		return
	}

	var req dto.UpdateSubscriptionTierRequest
	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(ctx, h.logger, w, "update subscription tier",
			fmt.Errorf("decode tier update: %w: %w", validator.ErrValidation, err))
		return
	}
	req = req.Normalize()
	if err := req.Validate(h.validator); err != nil {
		handleError(ctx, h.logger, w, "update subscription tier", err)
		return
	}

	tier, err := h.uc.UpdateSubscriptionTier(ctx, user.ID, tierID, req)
	if err != nil {
		handleError(ctx, h.logger, w, "update subscription tier", err)
		return
	}
	if tier == nil {
		logError(ctx, h.logger, "update subscription tier", errors.New("usecase returned nil tier"))
		response.ErrorInternal(w)
		return
	}

	response.WriteJSON(w, http.StatusOK, tier)
}

// GetSubscriptionOptions returns tiers and the reader's current subscription for an author.
// @Summary Варианты подписки автора
// @Description Возвращает текущую подписку читателя и доступные для оформления уровни страницами по возрастанию цены и ID. Для следующей страницы передайте next_cursor из ответа в параметр cursor. Персональный ответ не кэшируется.
// @Tags subscriptions
// @Produce json
// @Param author_id path string true "ID автора" Format(uuid)
// @Param post_id query string false "Ограничить уровнями, открывающими этот пост" Format(uuid)
// @Param limit query int false "Размер страницы (значения вне диапазона 1-50 заменяются на 20)" default(20) minimum(1) maximum(50)
// @Param cursor query string false "Курсор из next_cursor предыдущего ответа (для первой страницы не передаётся)"
// @Success 200 {object} dto.SubscriptionOptionsResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /authors/{author_id}/subscription-options [get]
func (h *SubscriptionHandler) GetSubscriptionOptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Cache-Control", "private, no-store")

	authorID, err := uuid.Parse(mux.Vars(r)["author_id"])
	if err != nil {
		handleError(ctx, h.logger, w, "get subscription options",
			fmt.Errorf("parse author id: %w: %w", validator.ErrValidation, err))
		return
	}

	query := r.URL.Query()
	var postID *uuid.UUID
	if query.Has("post_id") {
		rawPostID := query.Get("post_id")
		parsedPostID, err := uuid.Parse(rawPostID)
		if err != nil {
			handleError(ctx, h.logger, w, "get subscription options",
				fmt.Errorf("invalid post id: %w: %w", validator.ErrValidation, err))
			return
		}
		postID = &parsedPostID
	}

	readerID := uuid.Nil
	if user, ok := middleware.UserInfoFromContext(ctx); ok {
		readerID = user.ID
	}

	page, next, err := h.getSubscriptionTiers(ctx, readerID, authorID, postID, query, false)
	if err != nil {
		handleError(ctx, h.logger, w, "get subscription options", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, dto.SubscriptionOptionsResponse{
		CurrentSubscription: page.CurrentSubscription,
		Items:               page.Items,
		NextCursor:          next,
	})
}

func (h *SubscriptionHandler) getSubscriptionTiers(
	ctx context.Context,
	readerID, authorID uuid.UUID,
	postID *uuid.UUID,
	query url.Values,
	authorView bool,
) (*dto.SubscriptionTierPage, *string, error) {
	limit := feed.ParseLimit(query.Get(limitField), defaultLimit, maxLimit)
	cur, err := SubscriptionTiersCursor.Decode(query.Get(cursorFiled))
	if err != nil {
		return nil, nil, fmt.Errorf("decode subscription tier cursor: %w: %w", validator.ErrValidation, err)
	}

	var page *dto.SubscriptionTierPage
	if authorView {
		page, err = h.uc.GetMySubscriptionTiers(ctx, authorID, cur, limit)
	} else {
		page, err = h.uc.GetSubscriptionOptions(ctx, readerID, authorID, postID, cur, limit)
	}
	if err != nil {
		return nil, nil, err
	}
	if page == nil {
		return nil, nil, errors.New("get subscription tiers: usecase returned nil page")
	}

	next, err := SubscriptionTiersCursor.Encode(page.Next)
	if err != nil {
		if errors.Is(err, cursor.ErrInvalidCursor) {
			return nil, nil, fmt.Errorf("encode next subscription tier cursor: %w: %w", validator.ErrValidation, err)
		}
		return nil, nil, fmt.Errorf("encode next subscription tier cursor: %w", err)
	}

	if page.Items == nil {
		page.Items = make([]*dto.SubscriptionTier, 0)
	}
	if !authorView {
		for _, tier := range page.Items {
			tier.Status = ""
		}
	}
	return page, next, nil
}

// CreateSubscription subscribes to an author or upgrades the current subscription.
// @Summary Оформление или повышение подписки
// @Description Создаёт платёжный заказ и выполняет попытку оплаты выбранного уровня. Повтор request_key возвращает результат исходного действия.
// @Tags subscriptions
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param author_id path string true "ID автора" Format(uuid)
// @Param request body dto.CreateSubscriptionRequest true "Параметры подписки"
// @Success 200 {object} dto.CreateSubscriptionResponse "Повышение уровня"
// @Success 201 {object} dto.CreateSubscriptionResponse "Оформление подписки"
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /authors/{author_id}/subscriptions [post]
func (h *SubscriptionHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	authorID, err := uuid.Parse(mux.Vars(r)["author_id"])
	if err != nil {
		handleError(ctx, h.logger, w, "create subscription",
			fmt.Errorf("invalid author id: %w: %w", validator.ErrValidation, err))
		return
	}

	var req dto.CreateSubscriptionRequest
	if err := response.DecodeJSON(w, r, &req); err != nil {
		handleError(ctx, h.logger, w, "create subscription",
			fmt.Errorf("decode subscription request: %w: %w", validator.ErrValidation, err))
		return
	}
	req.AuthorID = authorID
	if err := req.Validate(h.validator); err != nil {
		handleError(ctx, h.logger, w, "create subscription", err)
		return
	}

	result, err := h.uc.CreateSubscription(ctx, user.ID, req)
	if err != nil {
		handleError(ctx, h.logger, w, "create subscription", err)
		return
	}
	if result == nil {
		handleError(ctx, h.logger, w, "create subscription", errors.New("usecase returned nil subscription"))
		return
	}

	statusCode := http.StatusCreated
	if req.Action == "upgrade" {
		statusCode = http.StatusOK
	}
	response.WriteJSON(w, statusCode, dto.CreateSubscriptionResponse{
		CurrentSubscription: *result,
	})
}

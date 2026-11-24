package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payment/dto"
	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payment/models"
	paymentsusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payment/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	paymentspb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/payment/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PaymentUsecase interface {
	CreatePayment(ctx context.Context, req dto.CreatePayment) (*models.PaymentOrder, error)
	GetPayments(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]*models.PaymentOrder, error)
	Pay(ctx context.Context, orderID uuid.UUID) (models.PaymentOrderStatus, error)
	CancelPayment(ctx context.Context, orderID uuid.UUID) (*models.PaymentOrder, error)
}

type Handler struct {
	paymentspb.UnimplementedPaymentServiceServer

	uc        PaymentUsecase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewHandler(uc PaymentUsecase, v *validator.Validator, logger *slog.Logger) *Handler {
	return &Handler{uc: uc, validator: v, logger: logger}
}

func (h *Handler) CreatePayment(ctx context.Context, req *paymentspb.CreatePaymentRequest) (*paymentspb.PaymentOrder, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	payerID, err := parseUUID(req.GetPayerId(), "payer_id")
	if err != nil {
		return nil, err
	}

	var expiresAt *time.Time
	if req.GetExpiresAt() != nil {
		if err := req.GetExpiresAt().CheckValid(); err != nil {
			return nil, status.Error(codes.InvalidArgument, "expires_at is invalid")
		}

		t := req.GetExpiresAt().AsTime()
		expiresAt = &t
	}

	input := dto.CreatePayment{
		PayerID:     payerID,
		RequestKey:  strings.TrimSpace(req.GetRequestKey()),
		Purpose:     strings.TrimSpace(req.GetPurpose()),
		Origin:      strings.TrimSpace(req.GetOrigin()),
		AmountMinor: req.GetAmountMinor(),
		Currency:    strings.TrimSpace(req.GetCurrency()),
		ExpiresAt:   expiresAt,
	}

	if err := input.Validate(h.validator); err != nil {
		return nil, h.handleError(ctx, "create payment", err)
	}

	result, err := h.uc.CreatePayment(ctx, input)
	if err != nil {
		return nil, h.handleError(ctx, "create payment", err)
	}

	return paymentOrderToProto(result)
}

func (h *Handler) GetPayments(ctx context.Context, req *paymentspb.GetPaymentsRequest) (*paymentspb.GetPaymentsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	rawIDs := req.GetPaymentOrderIds()
	orderIDs := make([]uuid.UUID, 0, len(rawIDs))

	for i, rawID := range rawIDs {
		id, err := parseUUID(rawID, fmt.Sprintf("payment_order_ids[%d]", i))
		if err != nil {
			return nil, err
		}

		orderIDs = append(orderIDs, id)
	}

	if len(orderIDs) == 0 {
		return &paymentspb.GetPaymentsResponse{
			Orders: []*paymentspb.PaymentOrder{},
		}, nil
	}

	results, err := h.uc.GetPayments(ctx, orderIDs)
	if err != nil {
		return nil, h.handleError(ctx, "get payments", err)
	}

	response := &paymentspb.GetPaymentsResponse{
		Orders: make([]*paymentspb.PaymentOrder, 0, len(results)),
	}

	for _, result := range results {
		payment, err := paymentOrderToProto(result)
		if err != nil {
			h.logger.ErrorContext(ctx, "failed to convert payment state",
				slog.String("op", "get payments"),
				slog.Any("error", err),
			)
			return nil, status.Error(codes.Internal, "internal error")
		}

		response.Orders = append(response.Orders, payment)
	}

	return response, nil
}

func (h *Handler) Pay(ctx context.Context, req *paymentspb.PayRequest) (*paymentspb.PayResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	orderID, err := parseUUID(req.GetPaymentOrderId(), "payment_order_id")
	if err != nil {
		return nil, err
	}

	paymentStatus, err := h.uc.Pay(ctx, orderID)
	if err != nil {
		return nil, h.handleError(ctx, "pay", err)
	}

	return &paymentspb.PayResponse{Status: string(paymentStatus)}, nil
}

func (h *Handler) CancelPayment(ctx context.Context, req *paymentspb.CancelPaymentRequest) (*paymentspb.PaymentOrder, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	orderID, err := uuid.Parse(req.GetPaymentOrderId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid payment_order_id")
	}

	order, err := h.uc.CancelPayment(ctx, orderID)
	if err != nil {
		return nil, h.handleError(ctx, "cancel payment", err)
	}

	return paymentOrderToProto(order)
}

func parseUUID(value, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Errorf(
			codes.InvalidArgument,
			"%s must be a valid non-zero UUID",
			field,
		)
	}

	return id, nil
}

func (h *Handler) handleError(ctx context.Context, op string, err error) error {
	h.logger.ErrorContext(ctx, "payment grpc error",
		slog.String("op", op),
		slog.Any("error", err),
	)

	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")

	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")

	case errors.Is(err, models.ErrPaymentOrderNotFound):
		return status.Error(codes.NotFound, "payment order not found")

	case errors.Is(err, models.ErrPaymentAttemptNotFound):
		return status.Error(codes.NotFound, "payment attempt not found")

	case errors.Is(err, models.ErrPaymentOrderValidation),
		errors.Is(err, models.ErrPaymentAttemptValidation),
		errors.Is(err, validator.ErrValidation),
		errors.Is(err, paymentsusecase.ErrValidation):
		return status.Error(codes.InvalidArgument, "invalid payment data")

	case errors.Is(err, models.ErrForbiddenUpdate),
		errors.Is(err, paymentsusecase.ErrForbidden):
		return status.Error(codes.FailedPrecondition, "payment cannot be updated")

	case errors.Is(err, paymentsusecase.ErrNotFound):
		return status.Error(codes.NotFound, "payment order or attempt not found")
	}

	return status.Error(codes.Internal, "internal error")
}

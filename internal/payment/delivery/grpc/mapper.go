package grpc

import (
	"errors"
	"time"

	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payment/models"
	paymentspb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/payment/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func paymentOrderToProto(order *models.PaymentOrder) (*paymentspb.PaymentOrder, error) {
	if order == nil {
		return nil, errors.New("payment order is nil")
	}

	result := &paymentspb.PaymentOrder{
		Id:                  order.ID.String(),
		PayerId:             order.PayerID.String(),
		Purpose:             string(order.Purpose),
		Origin:              string(order.Origin),
		ExpectedAmountMinor: order.ExpectedAmountMinor,
		Currency:            order.Currency,
		Status:              string(order.Status),
		CreditedAttemptId:   uuidPtrToString(order.CreditedAttemptID),
		PaidAt:              timePtrToTimestamp(order.PaidAt),
		CanceledAt:          timePtrToTimestamp(order.CanceledAt),
	}

	return result, nil
}

func uuidPtrToString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}

	value := id.String()
	return &value
}

func timePtrToTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}

	return timestamppb.New(*t)
}

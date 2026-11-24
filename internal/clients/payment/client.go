package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	paymentcontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/payment"
	paymentspb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/payment/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Config struct {
	Address string        `mapstructure:"address"`
	Timeout time.Duration `mapstructure:"timeout"`
}

func DefaultConfig() Config {
	return Config{
		Address: "localhost:50052",
		Timeout: time.Second,
	}
}

func (c Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.Address) == "" {
		errs = append(errs, errors.New("payment client: address must not be empty"))
	}
	if c.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("payment client: timeout must be positive, got %s", c.Timeout))
	}

	return errors.Join(errs...)
}

type Client struct {
	client  paymentspb.PaymentServiceClient
	timeout time.Duration
}

func New(conn grpc.ClientConnInterface, timeout time.Duration) *Client {
	return &Client{
		client:  paymentspb.NewPaymentServiceClient(conn),
		timeout: timeout,
	}
}

func (c *Client) CreatePayment(ctx context.Context, req paymentcontracts.CreatePaymentRequest) (*paymentcontracts.PaymentOrder, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.client.CreatePayment(ctx, &paymentspb.CreatePaymentRequest{
		PayerId:     req.PayerID.String(),
		RequestKey:  strings.TrimSpace(req.RequestKey),
		Purpose:     strings.TrimSpace(req.Purpose),
		Origin:      strings.TrimSpace(req.Origin),
		AmountMinor: req.AmountMinor,
		Currency:    strings.TrimSpace(req.Currency),
		ExpiresAt:   timestamppb.New(*req.ExpiresAt),
	})
	if err != nil {
		return nil, fmt.Errorf("payment service CreatePayment: %w", err)
	}

	order, err := parsePaymentOrder(resp)
	if err != nil {
		return nil, fmt.Errorf("payment service CreatePayment: %w", err)
	}

	return order, nil
}

func (c *Client) Pay(ctx context.Context, orderID uuid.UUID) (string, error) {
	if orderID == uuid.Nil {
		return "", errors.New("payment order ID is empty")
	}

	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.client.Pay(ctx, &paymentspb.PayRequest{
		PaymentOrderId: orderID.String(),
	})
	if err != nil {
		return "", fmt.Errorf("payment service Pay: %w", err)
	}
	if resp == nil {
		return "", errors.New("payment service returned empty Pay response")
	}

	status := strings.TrimSpace(resp.GetStatus())
	if status == "" {
		return "", errors.New("payment service returned empty payment status")
	}

	return status, nil
}

func (c *Client) CancelPayment(ctx context.Context, orderID uuid.UUID) (*paymentcontracts.PaymentOrder, error) {
	if orderID == uuid.Nil {
		return nil, errors.New("payment order ID is empty")
	}

	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.client.CancelPayment(ctx, &paymentspb.CancelPaymentRequest{
		PaymentOrderId: orderID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("payment service CancelPayment: %w", err)
	}

	order, err := parsePaymentOrder(resp)
	if err != nil {
		return nil, fmt.Errorf("payment service CancelPayment: %w", err)
	}

	if order.ID != orderID {
		return nil, fmt.Errorf("payment service returned unexpected order ID: got %s, want %s", order.ID, orderID)
	}

	return order, nil
}

func (c *Client) GetPayments(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]*paymentcontracts.PaymentOrder, error) {
	if len(orderIDs) == 0 {
		return map[uuid.UUID]*paymentcontracts.PaymentOrder{}, nil
	}

	uniqueIDs := make([]uuid.UUID, 0, len(orderIDs))
	seen := make(map[uuid.UUID]struct{}, len(orderIDs))

	for _, id := range orderIDs {
		if id == uuid.Nil {
			return nil, errors.New("payment order ID is empty")
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	requestIDs := make([]string, 0, len(uniqueIDs))
	for _, id := range uniqueIDs {
		requestIDs = append(requestIDs, id.String())
	}

	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.client.GetPayments(ctx, &paymentspb.GetPaymentsRequest{
		PaymentOrderIds: requestIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("payment service GetPayments: %w", err)
	}
	if resp == nil {
		return nil, errors.New("payment service returned empty GetPayments response")
	}

	result := make(map[uuid.UUID]*paymentcontracts.PaymentOrder, len(uniqueIDs))

	for _, pbOrder := range resp.GetOrders() {
		order, err := parsePaymentOrder(pbOrder)
		if err != nil {
			return nil, fmt.Errorf("payment service GetPayments: %w", err)
		}

		if _, requested := seen[order.ID]; !requested {
			return nil, fmt.Errorf("payment service returned unrequested order %s", order.ID)
		}

		if _, duplicate := result[order.ID]; duplicate {
			return nil, fmt.Errorf("payment service returned duplicate order %s", order.ID)
		}

		result[order.ID] = order
	}

	for _, id := range uniqueIDs {
		if _, exists := result[id]; !exists {
			return nil, fmt.Errorf("payment service did not return order %s", id)
		}
	}

	return result, nil
}

func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.timeout)
}

func parsePaymentOrder(pbOrder *paymentspb.PaymentOrder) (*paymentcontracts.PaymentOrder, error) {
	if pbOrder == nil {
		return nil, errors.New("payment service returned empty payment order")
	}

	orderID, err := uuid.Parse(strings.TrimSpace(pbOrder.GetId()))
	if err != nil {
		return nil, fmt.Errorf("invalid payment order ID: %w", err)
	}

	payerID, err := uuid.Parse(strings.TrimSpace(pbOrder.GetPayerId()))
	if err != nil {
		return nil, fmt.Errorf("invalid payment order payer ID: %w", err)
	}

	status := strings.TrimSpace(pbOrder.GetStatus())
	if status == "" {
		return nil, errors.New("payment service returned empty payment status")
	}

	var creditedAttemptID *uuid.UUID
	if value := strings.TrimSpace(pbOrder.GetCreditedAttemptId()); value != "" {
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("invalid credited attempt ID: %w", err)
		}
		creditedAttemptID = &id
	}

	paidAt, err := parseTimestamp(pbOrder.GetPaidAt())
	if err != nil {
		return nil, fmt.Errorf("invalid payment paid_at: %w", err)
	}

	canceledAt, err := parseTimestamp(pbOrder.GetCanceledAt())
	if err != nil {
		return nil, fmt.Errorf("invalid payment canceled_at: %w", err)
	}

	return &paymentcontracts.PaymentOrder{
		ID:                  orderID,
		PayerID:             payerID,
		Purpose:             pbOrder.GetPurpose(),
		Origin:              pbOrder.GetOrigin(),
		ExpectedAmountMinor: pbOrder.GetExpectedAmountMinor(),
		Currency:            pbOrder.GetCurrency(),
		Status:              status,
		CreditedAttemptID:   creditedAttemptID,
		PaidAt:              paidAt,
		CanceledAt:          canceledAt,
	}, nil
}

func parseTimestamp(ts *timestamppb.Timestamp) (*time.Time, error) {
	if ts == nil {
		return nil, nil
	}

	if err := ts.CheckValid(); err != nil {
		return nil, err
	}

	t := ts.AsTime()
	return &t, nil
}

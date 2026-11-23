package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	paymentspb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/payment/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
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
		errs = append(errs, errors.New("identity client: address must not be empty"))
	}
	if c.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("identity client: timeout must be positive, got %s", c.Timeout))
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

func (c *Client) Pay(ctx context.Context, orderID uuid.UUID) (string, error) {
	if orderID == uuid.Nil {
		return "", errors.New("payment order ID is empty")
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.Pay(ctx, &paymentspb.PayRequest{
		PaymentOrderId: orderID.String(),
	})
	if err != nil {
		return "", fmt.Errorf("payment service Pay: %w", err)
	}

	if resp == nil {
		return "", errors.New("payment service returned empty response")
	}

	paymentStatus := strings.TrimSpace(resp.GetStatus())
	if paymentStatus == "" {
		return "", errors.New("payment service returned empty payment status")
	}

	return paymentStatus, nil
}

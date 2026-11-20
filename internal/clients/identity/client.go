package identityclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Config struct {
	Address string        `mapstructure:"address"`
	Timeout time.Duration `mapstructure:"timeout"`
}

func DefaultConfig() Config {
	return Config{
		Address: "localhost:50051",
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
	client  identitypb.IdentityServiceClient
	timeout time.Duration
}

func New(conn grpc.ClientConnInterface, timeout time.Duration) *Client {
	return &Client{client: identitypb.NewIdentityServiceClient(conn), timeout: timeout}
}

func (c *Client) Verify(ctx context.Context, token string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.VerifyToken(ctx, &identitypb.VerifyTokenRequest{Token: token})
	if err != nil {
		switch status.Code(err) {
		case codes.Unauthenticated, codes.InvalidArgument:
			return nil, middleware.ErrInvalidToken
		default:
			return nil, fmt.Errorf("%w: %w", middleware.ErrAuthUnavailable, err)
		}
	}

	userID, err := uuid.Parse(resp.GetUserId())
	if err != nil {
		return nil, fmt.Errorf("%w: invalid user id: %w", middleware.ErrAuthUnavailable, err)
	}

	raw, err := json.Marshal(jwt.UserPayload{UserID: userID, Role: resp.GetRole()})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", middleware.ErrAuthUnavailable, err)
	}

	return raw, nil
}

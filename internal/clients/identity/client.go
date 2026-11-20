package identityclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/authn"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrUserNotFound = errors.New("user not found")

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

type UserInfo struct {
	ID        uuid.UUID
	Username  string
	Nickname  string
	AvatarURL string
	Status    string
	CreatedAt time.Time
}

type Client struct {
	client  identitypb.IdentityServiceClient
	timeout time.Duration
}

func New(conn grpc.ClientConnInterface, timeout time.Duration) *Client {
	return &Client{client: identitypb.NewIdentityServiceClient(conn), timeout: timeout}
}

func (c *Client) VerifyToken(ctx context.Context, token string) (*authn.User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.VerifyToken(ctx, &identitypb.VerifyTokenRequest{Token: token})
	if err != nil {
		switch status.Code(err) {
		case codes.Unauthenticated, codes.InvalidArgument:
			return nil, fmt.Errorf("%w: %w", authn.ErrInvalidToken, err)
		default:
			return nil, fmt.Errorf("%w: %w", authn.ErrUnavailable, err)
		}
	}

	userID, err := uuid.Parse(resp.GetUserId())
	if err != nil {
		return nil, fmt.Errorf("%w: invalid user id: %w", authn.ErrUnavailable, err)
	}

	return &authn.User{ID: userID, Role: resp.GetRole()}, nil
}

func (c *Client) GetUserByUsername(ctx context.Context, username string) (*UserInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetUserByUsername(ctx, &identitypb.GetUserByUsernameRequest{Username: username})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			return nil, ErrUserNotFound
		case codes.InvalidArgument:
			return nil, fmt.Errorf("identity client: invalid username: %w", err)
		default:
			return nil, fmt.Errorf("%w: %w", authn.ErrUnavailable, err)
		}
	}

	userID, err := uuid.Parse(resp.GetId())
	if err != nil {
		return nil, fmt.Errorf("%w: invalid user id: %w", authn.ErrUnavailable, err)
	}

	info := &UserInfo{
		ID:        userID,
		Username:  resp.GetUsername(),
		Nickname:  resp.GetNickname(),
		AvatarURL: resp.GetAvatarUrl(),
		Status:    resp.GetStatus(),
	}
	if created := resp.GetCreatedAt(); created != nil {
		info.CreatedAt = created.AsTime()
	}

	return info, nil
}

package identityclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	identitycontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/identity"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/authn"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrUnavailable  = errors.New("identity service unavailable")
	ErrUserNotFound = errors.New("user not found")
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

func (c *Client) GetUserByUsername(ctx context.Context, username string) (*identitycontracts.UserInfo, error) {
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

	info := &identitycontracts.UserInfo{
		ID:        userID,
		Username:  resp.GetUsername(),
		Nickname:  resp.GetNickname(),
		AvatarURL: resp.GetAvatarUrl(),
		Status:    resp.GetStatus(),
	}
	if created := resp.GetCreatedAt(); created != nil {
		c := created.AsTime()
		info.CreatedAt = &c
	}

	return info, nil
}

func (c *Client) GetUsers(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*identitycontracts.UserInfo, error) {
	requestIDs, err := toProtoIDs(ids)
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]*identitycontracts.UserInfo, len(requestIDs))
	if len(requestIDs) == 0 {
		return result, nil
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetUsers(ctx, &identitypb.GetUsersRequest{Ids: requestIDs})
	if err != nil {
		return nil, c.wrapRPCError("get users", err)
	}
	if resp == nil {
		return nil, errors.New("identity returned empty get users response")
	}

	for _, user := range resp.GetUsers() {
		if user == nil {
			return nil, errors.New("identity returned nil user")
		}

		id, err := uuid.Parse(user.GetId())
		if err != nil {
			return nil, fmt.Errorf("parse user id: %w", err)
		}

		var createdAt *time.Time
		if timestamp := user.GetCreatedAt(); timestamp != nil {
			if err := timestamp.CheckValid(); err != nil {
				return nil, fmt.Errorf("invalid user created_at: %w", err)
			}

			t := timestamp.AsTime()
			createdAt = &t
		}

		result[id] = &identitycontracts.UserInfo{
			ID:        id,
			Username:  user.GetUsername(),
			Nickname:  user.GetNickname(),
			AvatarURL: user.GetAvatarUrl(),
			CreatedAt: createdAt,
			Status:    user.GetStatus(),
		}
	}

	return result, nil
}

func toProtoIDs(ids []uuid.UUID) ([]string, error) {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	result := make([]string, 0, len(ids))

	for _, id := range ids {
		if id == uuid.Nil {
			return nil, errors.New("identity client: empty UUID in request")
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id.String())
	}

	return result, nil
}

func (c *Client) wrapRPCError(operation string, err error) error {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return fmt.Errorf(
			"%w: %s: %v",
			ErrUnavailable,
			operation,
			err,
		)
	default:
		return fmt.Errorf("identity %s: %w", operation, err)
	}
}

package identityclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/apperrors"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/dto"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	userIDHeader  = "x-user-id"
	avatarBufSize = 32 * 1024
)

type Client struct {
	client  identitypb.IdentityServiceClient
	timeout time.Duration
}

func New(conn *grpc.ClientConn, timeout time.Duration) *Client {
	return &Client{client: identitypb.NewIdentityServiceClient(conn), timeout: timeout}
}

func (c *Client) Register(ctx context.Context, req dto.RegistrationRequest) (*dto.AuthResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.Register(ctx, &identitypb.RegisterRequest{
		Email:    req.Email,
		Username: req.Username,
		Nickname: req.Nickname,
		Password: req.Password,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return toAuthResult(resp), nil
}

func (c *Client) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.Login(ctx, &identitypb.LoginRequest{Login: req.Login, Password: req.Password})
	if err != nil {
		return nil, mapError(err)
	}

	return toAuthResult(resp), nil
}

func (c *Client) GetMe(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	ctx = withUserID(ctx, userID)

	resp, err := c.client.GetMe(ctx, &identitypb.GetMeRequest{})
	if err != nil {
		return nil, mapError(err)
	}

	profile, err := toProfileResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", apperrors.ErrInternal, err)
	}

	return profile, nil
}

func (c *Client) GetPublicProfile(ctx context.Context, username string) (*dto.PublicProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetPublicProfile(ctx, &identitypb.GetPublicProfileRequest{Username: username})
	if err != nil {
		return nil, mapError(err)
	}

	profile, err := toPublicProfileResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", apperrors.ErrInternal, err)
	}

	return profile, nil
}

func (c *Client) UpdateMyProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) (*dto.ProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	ctx = withUserID(ctx, userID)

	update := &identitypb.UpdateMyProfileRequest{Nickname: req.Nickname}
	if req.Author != nil {
		update.Author = &identitypb.UpdateAuthorInput{Bio: req.Author.Bio, Category: req.Author.Category}
	}

	resp, err := c.client.UpdateMyProfile(ctx, update)
	if err != nil {
		return nil, mapError(err)
	}

	profile, err := toProfileResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", apperrors.ErrInternal, err)
	}

	return profile, nil
}

func (c *Client) BecomeAuthor(ctx context.Context, userID uuid.UUID, req dto.UpdateAuthorRequest) (*dto.ProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	ctx = withUserID(ctx, userID)

	resp, err := c.client.BecomeAuthor(ctx, &identitypb.BecomeAuthorRequest{Bio: req.Bio, Category: req.Category})
	if err != nil {
		return nil, mapError(err)
	}

	profile, err := toProfileResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", apperrors.ErrInternal, err)
	}

	return profile, nil
}

func (c *Client) UploadAvatar(ctx context.Context, userID uuid.UUID, file io.Reader, contentType string) (string, error) {
	ctx = withUserID(ctx, userID)

	stream, err := c.client.UploadAvatar(ctx)
	if err != nil {
		return "", mapError(err)
	}

	if err := stream.Send(&identitypb.UploadAvatarRequest{Payload: &identitypb.UploadAvatarRequest_Metadata{Metadata: &identitypb.AvatarMetadata{ContentType: contentType}}}); err != nil {
		if errors.Is(err, io.EOF) {
			_, recvErr := stream.CloseAndRecv()
			if recvErr == nil {
				recvErr = err
			}
			return "", mapError(recvErr)
		}
		return "", mapError(err)
	}

	buffer := make([]byte, avatarBufSize)
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			if err := stream.Send(&identitypb.UploadAvatarRequest{Payload: &identitypb.UploadAvatarRequest_Chunk{Chunk: buffer[:n]}}); err != nil {
				if errors.Is(err, io.EOF) {
					_, recvErr := stream.CloseAndRecv()
					if recvErr == nil {
						recvErr = err
					}
					return "", mapError(recvErr)
				}
				return "", mapError(err)
			}
		}

		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("%w: %v", apperrors.ErrBadRequest, readErr)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return "", mapError(err)
	}

	return resp.GetAvatarUrl(), nil
}

func (c *Client) DeleteAvatar(ctx context.Context, userID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	ctx = withUserID(ctx, userID)

	_, err := c.client.DeleteAvatar(ctx, &identitypb.DeleteAvatarRequest{})
	return mapError(err)
}

func withUserID(ctx context.Context, userID uuid.UUID) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		md = md.Copy()
	} else {
		md = metadata.MD{}
	}

	md.Set(userIDHeader, userID.String())
	return metadata.NewOutgoingContext(ctx, md)
}

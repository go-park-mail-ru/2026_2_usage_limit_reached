package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	authdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/dto"
	profiledto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/dto"
	profileuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthUsecase interface {
	Register(ctx context.Context, req authdto.RegistrationRequest) (*authdto.UserResponse, string, error)
	Login(ctx context.Context, req authdto.LoginRequest) (*authdto.UserResponse, string, error)
	TokenTTL() time.Duration
}

type ProfileUsecase interface {
	GetMyProfile(ctx context.Context, userID uuid.UUID) (*profiledto.ProfileResponse, error)
	GetPublicProfile(ctx context.Context, username string) (*profiledto.PublicProfileResponse, error)
	UpdateMyProfile(ctx context.Context, userID uuid.UUID, req profiledto.UpdateProfileRequest) (*profiledto.ProfileResponse, error)
	BecomeAuthor(ctx context.Context, userID uuid.UUID, req profiledto.UpdateAuthorRequest) (*profiledto.ProfileResponse, error)
	UploadAvatar(ctx context.Context, userID uuid.UUID, file io.ReadSeeker, size int64) (string, error)
	DeleteAvatar(ctx context.Context, userID uuid.UUID) error
}

type Handler struct {
	identitypb.UnimplementedIdentityServiceServer

	auth      AuthUsecase
	profile   ProfileUsecase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewHandler(auth AuthUsecase, profile ProfileUsecase, validator *validator.Validator, logger *slog.Logger) *Handler {
	return &Handler{
		auth:      auth,
		profile:   profile,
		validator: validator,
		logger:    logger,
	}
}

func (h *Handler) Register(ctx context.Context, req *identitypb.RegisterRequest) (*identitypb.AuthResponse, error) {
	reg := newRegistrationRequest(req)
	if err := reg.Validate(h.validator); err != nil {
		return nil, h.fail(ctx, "register", err)
	}

	user, token, err := h.auth.Register(ctx, reg)
	if err != nil {
		return nil, h.fail(ctx, "register", err)
	}

	return newAuthResponse(user, token, h.auth.TokenTTL()), nil
}

func (h *Handler) Login(ctx context.Context, req *identitypb.LoginRequest) (*identitypb.AuthResponse, error) {
	login := newLoginRequest(req)
	if err := login.Validate(); err != nil {
		return nil, h.fail(ctx, "login", err)
	}

	user, token, err := h.auth.Login(ctx, login)
	if err != nil {
		return nil, h.fail(ctx, "login", err)
	}

	return newAuthResponse(user, token, h.auth.TokenTTL()), nil
}

func (h *Handler) GetMe(ctx context.Context, _ *identitypb.GetMeRequest) (*identitypb.MyProfile, error) {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	profile, err := h.profile.GetMyProfile(ctx, userID)
	if err != nil {
		return nil, h.fail(ctx, "get my profile", err)
	}

	return newMyProfile(profile), nil
}

func (h *Handler) GetPublicProfile(ctx context.Context, req *identitypb.GetPublicProfileRequest) (*identitypb.PublicProfile, error) {
	if strings.TrimSpace(req.GetUsername()) == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	profile, err := h.profile.GetPublicProfile(ctx, req.GetUsername())
	if err != nil {
		return nil, h.fail(ctx, "get public profile", err)
	}

	return newPublicProfile(profile), nil
}

func (h *Handler) UpdateMyProfile(ctx context.Context, req *identitypb.UpdateMyProfileRequest) (*identitypb.MyProfile, error) {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	update := newUpdateProfileRequest(req)
	if err := update.Validate(h.validator); err != nil {
		return nil, h.fail(ctx, "update my profile", err)
	}

	profile, err := h.profile.UpdateMyProfile(ctx, userID, update)
	if err != nil {
		return nil, h.fail(ctx, "update my profile", err)
	}

	return newMyProfile(profile), nil
}

func (h *Handler) BecomeAuthor(ctx context.Context, req *identitypb.BecomeAuthorRequest) (*identitypb.MyProfile, error) {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	update := newBecomeAuthorRequest(req)
	if err := update.Validate(h.validator); err != nil {
		return nil, h.fail(ctx, "become author", err)
	}

	profile, err := h.profile.BecomeAuthor(ctx, userID, update)
	if err != nil {
		return nil, h.fail(ctx, "become author", err)
	}

	return newMyProfile(profile), nil
}

func (h *Handler) UploadAvatar(stream identitypb.IdentityService_UploadAvatarServer) error {
	ctx := stream.Context()
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return err
	}

	first, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return status.Error(codes.InvalidArgument, "metadata is required")
	}
	if err != nil {
		return status.Error(codes.Aborted, "upload aborted")
	}
	if first.GetMetadata() == nil {
		return status.Error(codes.InvalidArgument, "metadata must be first")
	}

	file, err := os.CreateTemp("", "avatar-*")
	if err != nil {
		return status.Error(codes.Internal, "cannot create temporary file")
	}
	defer os.Remove(file.Name())
	defer file.Close()

	var size int64
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return status.Error(codes.Aborted, "upload aborted")
		}
		if req.GetMetadata() != nil {
			return status.Error(codes.InvalidArgument, "metadata must be first")
		}

		chunk := req.GetChunk()
		if len(chunk) == 0 {
			continue
		}

		size += int64(len(chunk))
		if size > profileuc.MaxAvatarSize {
			return status.Error(codes.ResourceExhausted, "avatar too large")
		}
		if _, err := file.Write(chunk); err != nil {
			return status.Error(codes.Internal, "cannot write avatar")
		}
	}

	if size == 0 {
		return status.Error(codes.InvalidArgument, "avatar is empty")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return status.Error(codes.Internal, "cannot rewind avatar")
	}

	avatarURL, err := h.profile.UploadAvatar(ctx, userID, file, size)
	if err != nil {
		return h.fail(ctx, "upload avatar", err)
	}

	return stream.SendAndClose(&identitypb.AvatarResponse{AvatarUrl: avatarURL})
}

func (h *Handler) DeleteAvatar(ctx context.Context, _ *identitypb.DeleteAvatarRequest) (*identitypb.AvatarResponse, error) {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.profile.DeleteAvatar(ctx, userID); err != nil {
		return nil, h.fail(ctx, "delete avatar", err)
	}

	return &identitypb.AvatarResponse{}, nil
}

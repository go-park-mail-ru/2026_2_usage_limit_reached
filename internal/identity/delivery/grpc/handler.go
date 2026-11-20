package grpc

import (
	"context"
	"errors"
	"log/slog"

	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/dto"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/dto"
	userusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/authn"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserUsecase interface {
	FindUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]*userdto.UserInfo, error)
	FindUserByUsername(ctx context.Context, username string) (*userdto.UserInfo, error)
}

type AuthorUsecase interface {
	FindAuthorsByIDs(ctx context.Context, ids []uuid.UUID) ([]*authordto.AuthorInfo, error)
}

type AuthUsecase interface {
	VerifyToken(ctx context.Context, token string) (*authn.User, error)
}

type Handler struct {
	identitypb.UnimplementedIdentityServiceServer

	users   UserUsecase
	authors AuthorUsecase
	auth    AuthUsecase
	logger  *slog.Logger
}

func NewHandler(users UserUsecase, authors AuthorUsecase, auth AuthUsecase, logger *slog.Logger) *Handler {
	return &Handler{users: users, authors: authors, auth: auth, logger: logger}
}

func (h *Handler) GetUsers(ctx context.Context, req *identitypb.GetUsersRequest) (*identitypb.GetUsersResponse, error) {
	ids, err := parseIDs(req.GetIds())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	users, err := h.users.FindUsersByIDs(ctx, ids)
	if err != nil {
		h.logError(ctx, "get users", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return newGetUsersResponse(users), nil
}

func (h *Handler) GetUserByUsername(ctx context.Context, req *identitypb.GetUserByUsernameRequest) (*identitypb.UserInfo, error) {
	if req.GetUsername() == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	user, err := h.users.FindUserByUsername(ctx, req.GetUsername())
	if err != nil {
		if errors.Is(err, userusecase.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}

		h.logError(ctx, "get user by username", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	if user == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	return newUserInfo(user), nil
}

func (h *Handler) GetAuthors(ctx context.Context, req *identitypb.GetAuthorsRequest) (*identitypb.GetAuthorsResponse, error) {
	ids, err := parseIDs(req.GetIds())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid author id")
	}

	authors, err := h.authors.FindAuthorsByIDs(ctx, ids)
	if err != nil {
		h.logError(ctx, "get authors", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return newGetAuthorsResponse(authors), nil
}

func (h *Handler) VerifyToken(ctx context.Context, req *identitypb.VerifyTokenRequest) (*identitypb.VerifyTokenResponse, error) {
	if req.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	user, err := h.auth.VerifyToken(ctx, req.GetToken())
	if err != nil || user == nil || user.ID == uuid.Nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	return &identitypb.VerifyTokenResponse{
		UserId: user.ID.String(),
		Role:   user.Role,
	}, nil
}

func (h *Handler) logError(ctx context.Context, action string, err error) {
	if h.logger == nil {
		return
	}

	h.logger.ErrorContext(ctx, "identity grpc error",
		slog.String("handler", action),
		slog.String("error", err.Error()),
	)
}

func parseIDs(raw []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(raw))
	for _, value := range raw {
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

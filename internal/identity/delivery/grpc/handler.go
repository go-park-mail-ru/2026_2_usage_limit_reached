package grpc

import (
	"context"
	"encoding/json"
	"log/slog"

	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/dto"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserUsecase interface {
	FindUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]*userdto.UserInfo, error)
}

type AuthorUsecase interface {
	FindAuthorsByIDs(ctx context.Context, ids []uuid.UUID) ([]*authordto.AuthorInfo, error)
}

type TokenVerifier interface {
	Verify(ctx context.Context, token string) (json.RawMessage, error)
}

type Handler struct {
	identitypb.UnimplementedIdentityServiceServer

	users   UserUsecase
	authors AuthorUsecase
	tokens  TokenVerifier
	logger  *slog.Logger
}

func NewHandler(users UserUsecase, authors AuthorUsecase, tokens TokenVerifier, logger *slog.Logger) *Handler {
	return &Handler{users: users, authors: authors, tokens: tokens, logger: logger}
}

func (h *Handler) GetUsers(ctx context.Context, req *identitypb.GetUsersRequest) (*identitypb.GetUsersResponse, error) {
	ids, err := parseIDs(req.GetIds())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	users, err := h.users.FindUsersByIDs(ctx, ids)
	if err != nil {
		if h.logger != nil {
			h.logger.ErrorContext(ctx, "identity grpc error",
				slog.String("handler", "get users"),
				slog.String("error", err.Error()),
			)
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return newGetUsersResponse(users), nil
}

func (h *Handler) GetAuthors(ctx context.Context, req *identitypb.GetAuthorsRequest) (*identitypb.GetAuthorsResponse, error) {
	ids, err := parseIDs(req.GetIds())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid author id")
	}

	authors, err := h.authors.FindAuthorsByIDs(ctx, ids)
	if err != nil {
		if h.logger != nil {
			h.logger.ErrorContext(ctx, "identity grpc error",
				slog.String("handler", "get authors"),
				slog.String("error", err.Error()),
			)
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return newGetAuthorsResponse(authors), nil
}

func (h *Handler) VerifyToken(ctx context.Context, req *identitypb.VerifyTokenRequest) (*identitypb.VerifyTokenResponse, error) {
	if req.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	raw, err := h.tokens.Verify(ctx, req.GetToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	var payload jwt.UserPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.UserID == uuid.Nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	return &identitypb.VerifyTokenResponse{
		UserId: payload.UserID.String(),
		Role:   payload.Role,
	}, nil
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

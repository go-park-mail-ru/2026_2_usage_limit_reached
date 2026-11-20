package grpc_test

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	authuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/usecase"
	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/author/dto"
	identitygrpc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/grpc"
	mockgrpc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/grpc/mock_delivery_grpc"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/dto"
	userusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/user/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func newTestClient(
	t *testing.T,
	users identitygrpc.UserUsecase,
	authors identitygrpc.AuthorUsecase,
	auth identitygrpc.AuthUsecase,
) identitypb.IdentityServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	identitypb.RegisterIdentityServiceServer(server, identitygrpc.NewHandler(users, authors, auth, nil))

	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return identitypb.NewIdentityServiceClient(conn)
}

func TestGetUsers(t *testing.T) {
	userID := uuid.New()

	users := mockgrpc.NewMockUserUsecase(t)
	users.On("FindUsersByIDs", mock.Anything, mock.Anything).
		Return([]*userdto.UserInfo{{
			ID:       userID,
			Username: "user",
			Nickname: "nick",
			Status:   "active",
		}}, nil)

	client := newTestClient(t, users, mockgrpc.NewMockAuthorUsecase(t), nil)

	resp, err := client.GetUsers(context.Background(), &identitypb.GetUsersRequest{Ids: []string{userID.String()}})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.GetUsers()) != 1 {
		t.Fatalf("users count = %d", len(resp.GetUsers()))
	}
	if resp.GetUsers()[0].GetUsername() != "user" {
		t.Fatalf("username = %q", resp.GetUsers()[0].GetUsername())
	}
	if resp.GetUsers()[0].GetStatus() != "active" {
		t.Fatalf("status = %q", resp.GetUsers()[0].GetStatus())
	}
}

func TestGetAuthors(t *testing.T) {
	authorID := uuid.New()
	category := "Cars"

	authors := mockgrpc.NewMockAuthorUsecase(t)
	authors.On("FindAuthorsByIDs", mock.Anything, mock.Anything).
		Return([]*authordto.AuthorInfo{{
			ID:       authorID,
			Bio:      "bio",
			Category: &category,
		}}, nil)

	client := newTestClient(t, mockgrpc.NewMockUserUsecase(t), authors, nil)

	resp, err := client.GetAuthors(context.Background(), &identitypb.GetAuthorsRequest{Ids: []string{authorID.String()}})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.GetAuthors()) != 1 {
		t.Fatalf("authors count = %d", len(resp.GetAuthors()))
	}
	if resp.GetAuthors()[0].GetCategory() != category {
		t.Fatalf("category = %q", resp.GetAuthors()[0].GetCategory())
	}
}

func TestGetUsersInvalidID(t *testing.T) {
	client := newTestClient(t, mockgrpc.NewMockUserUsecase(t), mockgrpc.NewMockAuthorUsecase(t), nil)

	_, err := client.GetUsers(context.Background(), &identitypb.GetUsersRequest{Ids: []string{"not-a-uuid"}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestGetUserByUsername(t *testing.T) {
	userID := uuid.New()

	users := mockgrpc.NewMockUserUsecase(t)
	users.On("FindUserByUsername", mock.Anything, "user").
		Return(&userdto.UserInfo{
			ID:       userID,
			Username: "user",
			Nickname: "nick",
			Status:   "active",
		}, nil)

	client := newTestClient(t, users, mockgrpc.NewMockAuthorUsecase(t), nil)

	resp, err := client.GetUserByUsername(context.Background(), &identitypb.GetUserByUsernameRequest{Username: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetId() != userID.String() {
		t.Fatalf("user id = %q", resp.GetId())
	}
	if resp.GetUsername() != "user" {
		t.Fatalf("username = %q", resp.GetUsername())
	}
	if resp.GetNickname() != "nick" {
		t.Fatalf("nickname = %q", resp.GetNickname())
	}
	if resp.GetStatus() != "active" {
		t.Fatalf("status = %q", resp.GetStatus())
	}
}

func TestGetUserByUsernameNotFound(t *testing.T) {
	users := mockgrpc.NewMockUserUsecase(t)
	users.On("FindUserByUsername", mock.Anything, "ghost").
		Return(nil, userusecase.ErrUserNotFound)

	client := newTestClient(t, users, mockgrpc.NewMockAuthorUsecase(t), nil)

	_, err := client.GetUserByUsername(context.Background(), &identitypb.GetUserByUsernameRequest{Username: "ghost"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestGetUserByUsernameEmpty(t *testing.T) {
	client := newTestClient(t, mockgrpc.NewMockUserUsecase(t), mockgrpc.NewMockAuthorUsecase(t), nil)

	_, err := client.GetUserByUsername(context.Background(), &identitypb.GetUserByUsernameRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestVerifyToken(t *testing.T) {
	manager, err := jwt.NewJWTManager(jwt.JWTConfig{
		Secret:   strings.Repeat("a", 32),
		TokenTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	userID := uuid.New()
	token, err := manager.Generate(userID, jwt.UserPayload{UserID: userID, Role: "user"})
	if err != nil {
		t.Fatal(err)
	}

	client := newTestClient(t, nil, nil, authuc.NewUsecase(nil, manager))

	resp, err := client.VerifyToken(context.Background(), &identitypb.VerifyTokenRequest{Token: token})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetUserId() != userID.String() {
		t.Fatalf("user id = %q", resp.GetUserId())
	}
	if resp.GetRole() != "user" {
		t.Fatalf("role = %q", resp.GetRole())
	}

	_, err = client.VerifyToken(context.Background(), &identitypb.VerifyTokenRequest{Token: "not-a-token"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("invalid token code = %v", status.Code(err))
	}

	_, err = client.VerifyToken(context.Background(), &identitypb.VerifyTokenRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty token code = %v", status.Code(err))
	}
}

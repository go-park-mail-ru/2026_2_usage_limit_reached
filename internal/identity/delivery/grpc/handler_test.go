package grpc_test

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	identitygrpc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/grpc"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	identityuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/authn"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type testJWTPayload struct {
	UserID uuid.UUID
	Role   string
}

func (p testJWTPayload) Encode() ([]byte, error) {
	return json.Marshal(map[string]any{"user_id": p.UserID.String(), "role": p.Role})
}

type stubIdentity struct {
	getUsers          func(ctx context.Context, ids []uuid.UUID) ([]*dto.UserInfo, error)
	getUserByUsername func(ctx context.Context, username string) (*dto.UserInfo, error)
	verifyToken       func(ctx context.Context, token string) (*authn.User, error)
}

func (s stubIdentity) GetUsers(ctx context.Context, ids []uuid.UUID) ([]*dto.UserInfo, error) {
	if s.getUsers != nil {
		return s.getUsers(ctx, ids)
	}

	return nil, nil
}

func (s stubIdentity) GetUserByUsername(ctx context.Context, username string) (*dto.UserInfo, error) {
	if s.getUserByUsername != nil {
		return s.getUserByUsername(ctx, username)
	}

	return nil, nil
}

func (s stubIdentity) VerifyToken(ctx context.Context, token string) (*authn.User, error) {
	if s.verifyToken != nil {
		return s.verifyToken(ctx, token)
	}

	return nil, authn.ErrInvalidToken
}

func newTestClient(t *testing.T, identity identitygrpc.IdentityUsecase) identitypb.IdentityServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	identitypb.RegisterIdentityServiceServer(server, identitygrpc.NewHandler(identity, nil))

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

	client := newTestClient(t, stubIdentity{
		getUsers: func(context.Context, []uuid.UUID) ([]*dto.UserInfo, error) {
			return []*dto.UserInfo{{
				ID:       userID,
				Username: "user",
				Nickname: "nick",
				Status:   "active",
			}}, nil
		},
	})

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

func TestGetUsersInvalidID(t *testing.T) {
	client := newTestClient(t, stubIdentity{})

	_, err := client.GetUsers(context.Background(), &identitypb.GetUsersRequest{Ids: []string{"not-a-uuid"}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestGetUserByUsername(t *testing.T) {
	userID := uuid.New()

	client := newTestClient(t, stubIdentity{
		getUserByUsername: func(context.Context, string) (*dto.UserInfo, error) {
			return &dto.UserInfo{
				ID:       userID,
				Username: "user",
				Nickname: "nick",
				Status:   "active",
			}, nil
		},
	})

	resp, err := client.GetUserByUsername(context.Background(), &identitypb.GetUserByUsernameRequest{Username: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetId() != userID.String() {
		t.Fatalf("user id = %q", resp.GetId())
	}
	if resp.GetNickname() != "nick" {
		t.Fatalf("nickname = %q", resp.GetNickname())
	}
}

func TestGetUserByUsernameNotFound(t *testing.T) {
	client := newTestClient(t, stubIdentity{
		getUserByUsername: func(context.Context, string) (*dto.UserInfo, error) {
			return nil, identityuc.ErrUserNotFound
		},
	})

	_, err := client.GetUserByUsername(context.Background(), &identitypb.GetUserByUsernameRequest{Username: "ghost"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestGetUserByUsernameEmpty(t *testing.T) {
	client := newTestClient(t, stubIdentity{})

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
	token, err := manager.Generate(userID, testJWTPayload{UserID: userID, Role: "user"})
	if err != nil {
		t.Fatal(err)
	}

	authUc := identityuc.New(nil, manager, nil, "", nil)
	client := newTestClient(t, stubIdentity{verifyToken: authUc.VerifyToken})

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

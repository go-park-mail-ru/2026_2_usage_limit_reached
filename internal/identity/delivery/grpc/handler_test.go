package grpc_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	authdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/dto"
	authuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/auth/usecase"
	identitygrpc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/grpc"
	profiledto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	identitypb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/identity/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeAuth struct {
	user  *authdto.UserResponse
	token string
	err   error
}

func (f *fakeAuth) Register(context.Context, authdto.RegistrationRequest) (*authdto.UserResponse, string, error) {
	return f.user, f.token, f.err
}

func (f *fakeAuth) Login(context.Context, authdto.LoginRequest) (*authdto.UserResponse, string, error) {
	return f.user, f.token, f.err
}

func (f *fakeAuth) TokenTTL() time.Duration {
	return time.Hour
}

type fakeProfile struct {
	myProfile *profiledto.ProfileResponse
	public    *profiledto.PublicProfileResponse
	avatarURL string
}

func (f *fakeProfile) GetMyProfile(context.Context, uuid.UUID) (*profiledto.ProfileResponse, error) {
	return f.myProfile, nil
}

func (f *fakeProfile) GetPublicProfile(context.Context, string) (*profiledto.PublicProfileResponse, error) {
	return f.public, nil
}

func (f *fakeProfile) UpdateMyProfile(context.Context, uuid.UUID, profiledto.UpdateProfileRequest) (*profiledto.ProfileResponse, error) {
	return f.myProfile, nil
}

func (f *fakeProfile) BecomeAuthor(context.Context, uuid.UUID, profiledto.UpdateAuthorRequest) (*profiledto.ProfileResponse, error) {
	return f.myProfile, nil
}

func (f *fakeProfile) UploadAvatar(context.Context, uuid.UUID, io.ReadSeeker, int64) (string, error) {
	return f.avatarURL, nil
}

func (f *fakeProfile) DeleteAvatar(context.Context, uuid.UUID) error {
	return nil
}

func newTestClient(t *testing.T, auth *fakeAuth, profile *fakeProfile) identitypb.IdentityServiceClient {
	t.Helper()

	valid, err := validator.NewValidator(validator.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(identitygrpc.UserContextInterceptor),
		grpc.ChainStreamInterceptor(identitygrpc.UserContextStreamInterceptor),
	)
	identitypb.RegisterIdentityServiceServer(server, identitygrpc.NewHandler(auth, profile, valid, nil))

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

func TestRegister(t *testing.T) {
	client := newTestClient(t,
		&fakeAuth{user: &authdto.UserResponse{Username: "user", Nickname: "nick"}, token: "access"},
		&fakeProfile{},
	)

	resp, err := client.Register(context.Background(), &identitypb.RegisterRequest{
		Email:    "user@example.com",
		Username: "user",
		Nickname: "nick",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetAccessToken() != "access" {
		t.Fatalf("access token = %q", resp.GetAccessToken())
	}
}

func TestGetMe(t *testing.T) {
	userID := uuid.New()
	client := newTestClient(t,
		&fakeAuth{},
		&fakeProfile{myProfile: &profiledto.ProfileResponse{
			User: profiledto.ProfileUserResponse{
				PublicProfileUserResponse: profiledto.PublicProfileUserResponse{ID: userID, Username: "user", Nickname: "nick"},
				Email:                     "user@example.com",
			},
		}},
	)

	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-user-id", userID.String())
	resp, err := client.GetMe(ctx, &identitypb.GetMeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetEmail() != "user@example.com" {
		t.Fatalf("email = %q", resp.GetEmail())
	}
}

func TestGetPublicProfileEmptyUsername(t *testing.T) {
	client := newTestClient(t, &fakeAuth{}, &fakeProfile{})

	_, err := client.GetPublicProfile(context.Background(), &identitypb.GetPublicProfileRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestGetMeWithoutUserID(t *testing.T) {
	client := newTestClient(t, &fakeAuth{}, &fakeProfile{})

	_, err := client.GetMe(context.Background(), &identitypb.GetMeRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	client := newTestClient(t, &fakeAuth{err: authuc.ErrLoginFailed}, &fakeProfile{})

	_, err := client.Login(context.Background(), &identitypb.LoginRequest{Login: "user", Password: "secret"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestUploadAvatarWithoutMetadata(t *testing.T) {
	userID := uuid.New()
	client := newTestClient(t, &fakeAuth{}, &fakeProfile{})

	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-user-id", userID.String())
	stream, err := client.UploadAvatar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&identitypb.UploadAvatarRequest{Payload: &identitypb.UploadAvatarRequest_Chunk{Chunk: []byte("avatar")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.CloseAndRecv(); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestUploadAvatar(t *testing.T) {
	userID := uuid.New()
	client := newTestClient(t,
		&fakeAuth{},
		&fakeProfile{avatarURL: "/media/avatar.png"},
	)

	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-user-id", userID.String())
	stream, err := client.UploadAvatar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&identitypb.UploadAvatarRequest{Payload: &identitypb.UploadAvatarRequest_Metadata{Metadata: &identitypb.AvatarMetadata{ContentType: "image/png"}}}); err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&identitypb.UploadAvatarRequest{Payload: &identitypb.UploadAvatarRequest_Chunk{Chunk: []byte("avatar")}}); err != nil {
		t.Fatal(err)
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetAvatarUrl() != "/media/avatar.png" {
		t.Fatalf("avatar url = %q", resp.GetAvatarUrl())
	}
}

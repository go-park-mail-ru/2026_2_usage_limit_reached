package onlineconf

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	contracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/onlineconf"
	onlineconfpb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/onlineconf/v1"
)

// fakeServer — минимальная реализация сервиса: отдаёт заданный ответ или ошибку
// и запоминает присланные запросы.
type fakeServer struct {
	onlineconfpb.UnimplementedOnlineConfServiceServer

	mu       sync.Mutex
	resp     *onlineconfpb.GetSnapshotResponse
	err      error
	requests []*onlineconfpb.GetSnapshotRequest
}

func (s *fakeServer) set(resp *onlineconfpb.GetSnapshotResponse, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resp, s.err = resp, err
}

func (s *fakeServer) lastRequest() *onlineconfpb.GetSnapshotRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests[len(s.requests)-1]
}

func (s *fakeServer) GetSnapshot(_ context.Context, req *onlineconfpb.GetSnapshotRequest) (*onlineconfpb.GetSnapshotResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req)
	return s.resp, s.err
}

func newTestClient(t *testing.T, scopes ...string) (*Client, *fakeServer) {
	t.Helper()

	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	fake := &fakeServer{}
	onlineconfpb.RegisterOnlineConfServiceServer(server, fake)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	cfg := DefaultConfig()
	cfg.Scopes = scopes
	cfg.PollInterval = 10 * time.Millisecond

	return New(conn, cfg, slog.New(slog.NewTextHandler(io.Discard, nil))), fake
}

func TestClientRefreshBuildsSnapshot(t *testing.T) {
	// Замороженное место ключа payment.min_amount_minor — скоуп monetization.
	client, fake := newTestClient(t, "content", "content", "monetization")
	user := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	banned := uuid.New()

	fake.set(&onlineconfpb.GetSnapshotResponse{
		Revision: 7,
		Configs: []*onlineconfpb.Config{
			{Scope: "monetization", Key: "payment.min_amount_minor", Value: "5000"},
			{Scope: "monetization", Key: "broken.int", Value: "abc"},
		},
		Flags: []*onlineconfpb.Flag{{
			Scope:           "content",
			Key:             "feed_attachment_filter",
			RolloutPercent:  0,
			ForceOnUserIds:  []string{user.String(), "not-a-uuid"}, // невалидный id пропускается
			ForceOffUserIds: []string{banned.String()},
		}},
	}, nil)

	if err := client.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if got := fake.lastRequest(); len(got.GetScopes()) != 2 || got.GetKnownRevision() != 0 {
		t.Errorf("first request = %+v, want 2 scopes and known_revision 0", got)
	}
	if client.Revision() != 7 || client.LastSync().IsZero() {
		t.Errorf("revision = %d, last sync = %v", client.Revision(), client.LastSync())
	}

	if got := contracts.Int64(client, "monetization", "payment.min_amount_minor", 100); got != 5000 {
		t.Errorf("min amount = %d, want 5000", got)
	}
	if got := contracts.Int64(client, "monetization", "broken.int", 100); got != 100 {
		t.Errorf("unparsable value should fall back to default, got %d", got)
	}
	if got := contracts.Int64(client, "monetization", "missing", 100); got != 100 {
		t.Errorf("missing key should fall back to default, got %d", got)
	}
	if _, ok := client.Value("content", "payment.min_amount_minor"); ok {
		t.Error("value from another scope must not be visible")
	}

	if !client.Enabled("content", "feed_attachment_filter", user) {
		t.Error("force-on user should be enabled")
	}
	if client.Enabled("content", "feed_attachment_filter", banned) {
		t.Error("force-off user should be disabled")
	}
	if client.Enabled("content", "feed_attachment_filter", uuid.Nil) {
		t.Error("anonymous should be disabled")
	}
	if got := client.Decide("content", "unknown_flag", user); got.Reason != contracts.ReasonFlagNotFound || got.Enabled {
		t.Errorf("unknown flag decision = %+v", got)
	}
}

func TestClientNotModifiedKeepsSnapshot(t *testing.T) {
	client, fake := newTestClient(t, "monetization")

	fake.set(&onlineconfpb.GetSnapshotResponse{
		Revision: 3,
		Configs:  []*onlineconfpb.Config{{Scope: "monetization", Key: "k", Value: "v1"}},
	}, nil)
	if err := client.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	fake.set(&onlineconfpb.GetSnapshotResponse{Revision: 3, NotModified: true}, nil)
	if err := client.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh (not modified): %v", err)
	}

	if got := fake.lastRequest().GetKnownRevision(); got != 3 {
		t.Errorf("known_revision = %d, want 3", got)
	}
	if v, _ := client.Value("monetization", "k"); v != "v1" {
		t.Errorf("snapshot lost after not_modified: %q", v)
	}

	fake.set(&onlineconfpb.GetSnapshotResponse{
		Revision: 4,
		Configs:  []*onlineconfpb.Config{{Scope: "monetization", Key: "k", Value: "v2"}},
	}, nil)
	if err := client.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh (update): %v", err)
	}
	if v, _ := client.Value("monetization", "k"); v != "v2" || client.Revision() != 4 {
		t.Errorf("snapshot not updated: value %q, revision %d", v, client.Revision())
	}
}

func TestClientErrorsKeepSnapshot(t *testing.T) {
	client, fake := newTestClient(t, "monetization")

	fake.set(&onlineconfpb.GetSnapshotResponse{
		Revision: 1,
		Configs:  []*onlineconfpb.Config{{Scope: "monetization", Key: "k", Value: "v"}},
	}, nil)
	if err := client.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	fake.set(nil, status.Error(codes.Unavailable, "down"))
	if err := client.Refresh(context.Background()); !errors.Is(err, contracts.ErrUnavailable) {
		t.Errorf("Unavailable should map to ErrUnavailable, got %v", err)
	}

	fake.set(nil, status.Error(codes.InvalidArgument, "no scopes"))
	if err := client.Refresh(context.Background()); !errors.Is(err, contracts.ErrInvalidScopes) {
		t.Errorf("InvalidArgument should map to ErrInvalidScopes, got %v", err)
	}

	if v, _ := client.Value("monetization", "k"); v != "v" {
		t.Errorf("snapshot must survive errors, got %q", v)
	}
}

func TestClientRunPollsUntilCancelled(t *testing.T) {
	client, fake := newTestClient(t, "monetization")
	fake.set(&onlineconfpb.GetSnapshotResponse{
		Revision: 1,
		Configs:  []*onlineconfpb.Config{{Scope: "monetization", Key: "k", Value: "v"}},
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		client.Run(ctx)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for client.Revision() != 1 {
		select {
		case <-deadline:
			t.Fatal("Run did not load the snapshot in time")
		case <-time.After(5 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func TestConfigValidate(t *testing.T) {
	valid := DefaultConfig()
	valid.Scopes = []string{"content"}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid config: %v", err)
	}

	invalid := Config{Address: " ", Timeout: 0, PollInterval: -1, Scopes: []string{""}}
	if err := invalid.Validate(); err == nil {
		t.Error("invalid config must fail validation")
	}
	if err := DefaultConfig().Validate(); err == nil {
		t.Error("config without scopes must fail validation")
	}
}

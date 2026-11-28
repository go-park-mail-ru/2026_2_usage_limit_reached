package grpc

import (
	"context"
	"log/slog"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/models"
	onlineconfpb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/onlineconf/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OnlineConfUsecase interface {
	GetSnapshot(ctx context.Context, scopes []string, knownRevision int64) (models.Snapshot, error)
}

type Handler struct {
	onlineconfpb.UnimplementedOnlineConfServiceServer

	onlineconf OnlineConfUsecase
	logger     *slog.Logger
}

func NewHandler(onlineconf OnlineConfUsecase, logger *slog.Logger) *Handler {
	return &Handler{onlineconf: onlineconf, logger: logger}
}

func (h *Handler) GetSnapshot(ctx context.Context, req *onlineconfpb.GetSnapshotRequest) (*onlineconfpb.GetSnapshotResponse, error) {
	scopes := req.GetScopes()
	if len(scopes) == 0 || hasBlank(scopes) {
		return nil, status.Error(codes.InvalidArgument, "scopes must not be empty")
	}

	snap, err := h.onlineconf.GetSnapshot(ctx, unique(scopes), req.GetKnownRevision())
	if err != nil {
		h.logError(ctx, "get snapshot", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	if snap.NotModified {
		return &onlineconfpb.GetSnapshotResponse{Revision: snap.Revision, NotModified: true}, nil
	}

	resp := &onlineconfpb.GetSnapshotResponse{
		Revision: snap.Revision,
		Configs:  make([]*onlineconfpb.Config, 0, len(snap.Configs)),
		Flags:    make([]*onlineconfpb.Flag, 0, len(snap.Flags)),
	}

	for _, cfg := range snap.Configs {
		resp.Configs = append(resp.Configs, &onlineconfpb.Config{
			Scope: cfg.Scope,
			Key:   cfg.Key,
			Value: cfg.Value,
		})
	}

	for _, flag := range snap.Flags {
		forceOn := make([]string, 0, len(flag.ForceOn))
		for _, id := range flag.ForceOn {
			forceOn = append(forceOn, id.String())
		}

		forceOff := make([]string, 0, len(flag.ForceOff))
		for _, id := range flag.ForceOff {
			forceOff = append(forceOff, id.String())
		}

		resp.Flags = append(resp.Flags, &onlineconfpb.Flag{
			Scope:           flag.Scope,
			Key:             flag.Key,
			RolloutPercent:  uint32(flag.RolloutPercent),
			ForceOnUserIds:  forceOn,
			ForceOffUserIds: forceOff,
		})
	}

	return resp, nil
}

func (h *Handler) logError(ctx context.Context, action string, err error) {
	if h.logger == nil {
		return
	}

	h.logger.ErrorContext(ctx, "onlineconf grpc error",
		slog.String("handler", action),
		slog.String("error", err.Error()),
	)
}

func hasBlank(scopes []string) bool {
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == "" {
			return true
		}
	}
	return false
}

func unique(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes))
	result := make([]string, 0, len(scopes))

	for _, scope := range scopes {
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}

	return result
}

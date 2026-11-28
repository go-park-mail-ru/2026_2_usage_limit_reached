package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/models"
)

// fakeRepo — минимальная реализация Repository для юнит-тестов usecase.
type fakeRepo struct {
	canManage bool

	snapshot models.Snapshot

	flags map[string]models.Flag // ключ "scope/key"

	rolloutUpdates int
	lastRollout     int16
}

func (f *fakeRepo) GetSnapshot(_ context.Context, _ []string, knownRevision int64) (models.Snapshot, error) {
	if knownRevision > 0 && knownRevision == f.snapshot.Revision {
		return models.Snapshot{Revision: f.snapshot.Revision, NotModified: true}, nil
	}
	return f.snapshot, nil
}

func (f *fakeRepo) CanManageOnlineConf(_ context.Context, _ uuid.UUID) (bool, error) {
	return f.canManage, nil
}

func (f *fakeRepo) ListScopes(_ context.Context) ([]models.Scope, error) { return nil, nil }

func (f *fakeRepo) CreateScope(_ context.Context, _ uuid.UUID, _ models.Scope) (*models.Scope, error) {
	return nil, nil
}

func (f *fakeRepo) ListConfigs(_ context.Context, _, _ string) ([]models.Config, error) {
	return nil, nil
}

func (f *fakeRepo) GetConfig(_ context.Context, _, _ string) (*models.Config, error) {
	return nil, models.ErrConfigNotFound
}

func (f *fakeRepo) CreateConfig(_ context.Context, _ uuid.UUID, _ models.Config) (*models.Config, error) {
	return nil, nil
}

func (f *fakeRepo) UpdateConfig(_ context.Context, _ uuid.UUID, _, _ string, _ models.Config) (*models.Config, error) {
	return nil, nil
}

func (f *fakeRepo) DeleteConfig(_ context.Context, _ uuid.UUID, _, _ string) error { return nil }

func (f *fakeRepo) ListFlags(_ context.Context, _, _ string) ([]models.Flag, error) {
	return nil, nil
}

func (f *fakeRepo) GetFlag(_ context.Context, scope, key string) (*models.Flag, error) {
	flag, ok := f.flags[scope+"/"+key]
	if !ok {
		return nil, models.ErrFlagNotFound
	}
	return &flag, nil
}

func (f *fakeRepo) CreateFlag(_ context.Context, _ uuid.UUID, _ models.Flag) (*models.Flag, error) {
	return nil, nil
}

func (f *fakeRepo) UpdateRollout(_ context.Context, _ uuid.UUID, _, _ string, percent int16) (*models.Flag, error) {
	f.rolloutUpdates++
	f.lastRollout = percent
	return &models.Flag{RolloutPercent: percent}, nil
}

func (f *fakeRepo) SetForce(_ context.Context, _ uuid.UUID, _, _ string, _ uuid.UUID, _ string) error {
	return nil
}

func (f *fakeRepo) RemoveForce(_ context.Context, _ uuid.UUID, _, _ string, _ uuid.UUID) error {
	return nil
}

func (f *fakeRepo) DeleteFlag(_ context.Context, _ uuid.UUID, _, _ string) error { return nil }

func (f *fakeRepo) ListHistory(_ context.Context, _ models.HistoryFilter, _ *int64, _ int) ([]models.HistoryEntry, *int64, error) {
	return nil, nil, nil
}

func newTestUsecase(t *testing.T, repo Repository) *Usecase {
	t.Helper()
	return New(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestValidateValue(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		valueType string
		wantErr   bool
	}{
		{"scalar number", `300`, models.ValueTypeScalar, false},
		{"scalar string", `"300"`, models.ValueTypeScalar, false},
		{"scalar bool", `true`, models.ValueTypeScalar, false},
		{"scalar object rejected", `{"min":1}`, models.ValueTypeScalar, true},
		{"scalar array rejected", `[1,2]`, models.ValueTypeScalar, true},
		{"array ok", `[1,2,3]`, models.ValueTypeArray, false},
		{"array object rejected", `{"a":1}`, models.ValueTypeArray, true},
		{"json object ok", `{"min":10,"max":100}`, models.ValueTypeJSON, false},
		{"json scalar rejected", `300`, models.ValueTypeJSON, true},
		{"broken json", `{min:10}`, models.ValueTypeJSON, true},
		{"unknown type", `300`, "yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateValue(tt.value, tt.valueType)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateValue(%q, %q) error = %v, wantErr %v", tt.value, tt.valueType, err, tt.wantErr)
			}
		})
	}
}

func TestCreateConfigInvalidValue(t *testing.T) {
	uc := newTestUsecase(t, &fakeRepo{})

	_, err := uc.CreateConfig(context.Background(), uuid.New(), "front", "k", `{"min":1}`, models.ValueTypeScalar, "")
	if err == nil {
		t.Fatal("object value with scalar type must be rejected")
	}
	if !errors.Is(err, models.ErrInvalidValue) {
		t.Errorf("error = %v, want models.ErrInvalidValue", err)
	}
}

func TestUpdateRolloutBounds(t *testing.T) {
	repo := &fakeRepo{}
	uc := newTestUsecase(t, repo)

	for _, percent := range []int{-1, 101} {
		if _, err := uc.UpdateRollout(context.Background(), uuid.New(), "content", "f", percent); err == nil {
			t.Errorf("percent %d must be rejected", percent)
		}
	}
	if repo.rolloutUpdates != 0 {
		t.Fatalf("rejected percents must not reach repository, got %d calls", repo.rolloutUpdates)
	}

	if _, err := uc.UpdateRollout(context.Background(), uuid.New(), "content", "f", 30); err != nil {
		t.Fatalf("valid percent: %v", err)
	}
	if repo.lastRollout != 30 {
		t.Errorf("lastRollout = %d, want 30", repo.lastRollout)
	}
}

func TestCheckFlag(t *testing.T) {
	user := uuid.MustParse("11111111-1111-1111-1111-111111111111") // bucket 1610 для feed_attachment_filter
	other := uuid.New()

	repo := &fakeRepo{flags: map[string]models.Flag{
		"content/feed_attachment_filter": {
			Scope: "content", Key: "feed_attachment_filter",
			RolloutPercent: 17, // bucket 1610 включён начиная с 17%
			ForceOff:       []uuid.UUID{user},
		},
		"content/simple": {
			Scope: "content", Key: "simple", RolloutPercent: 100,
		},
	}}
	uc := newTestUsecase(t, repo)

	// force_off сильнее раскатки.
	decision, err := uc.CheckFlag(context.Background(), "content", "feed_attachment_filter", user)
	if err != nil {
		t.Fatalf("check flag: %v", err)
	}
	if decision.Enabled || decision.Reason != "force_off" {
		t.Errorf("decision = %+v, want force_off disabled", decision)
	}

	// Юзер вне force-списков попадает под раскатку.
	decision, err = uc.CheckFlag(context.Background(), "content", "simple", other)
	if err != nil {
		t.Fatalf("check flag: %v", err)
	}
	if !decision.Enabled || decision.Reason != "rollout" {
		t.Errorf("decision = %+v, want rollout enabled", decision)
	}

	// Отсутствующий флаг — не ошибка, а reason flag_not_found.
	decision, err = uc.CheckFlag(context.Background(), "content", "unknown", other)
	if err != nil {
		t.Fatalf("check flag: %v", err)
	}
	if decision.Enabled || decision.Reason != "flag_not_found" {
		t.Errorf("decision = %+v, want flag_not_found", decision)
	}
}

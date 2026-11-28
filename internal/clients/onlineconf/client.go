package onlineconf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	contracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/onlineconf"
	onlineconfpb "github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/proto/onlineconf/v1"
)

type Config struct {
	Address string        `mapstructure:"address"`
	Timeout time.Duration `mapstructure:"timeout"`
	// PollInterval — период опроса сервиса. Новая настройка применяется у потребителя
	// не позже чем через PollInterval + Timeout после сохранения в админке.
	PollInterval time.Duration `mapstructure:"poll_interval"`
	// Scopes — скоупы, которые нужны этому сервису, например ["content"].
	Scopes []string `mapstructure:"scopes"`
}

func DefaultConfig() Config {
	return Config{
		Address:      "localhost:50052",
		Timeout:      time.Second,
		PollInterval: 15 * time.Second,
	}
}

func (c Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.Address) == "" {
		errs = append(errs, errors.New("onlineconf client: address must not be empty"))
	}
	if c.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("onlineconf client: timeout must be positive, got %s", c.Timeout))
	}
	if c.PollInterval <= 0 {
		errs = append(errs, fmt.Errorf("onlineconf client: poll_interval must be positive, got %s", c.PollInterval))
	}
	if len(c.Scopes) == 0 {
		errs = append(errs, errors.New("onlineconf client: scopes must not be empty"))
	}
	for _, scope := range c.Scopes {
		if strings.TrimSpace(scope) == "" {
			errs = append(errs, errors.New("onlineconf client: scope must not be empty"))
			break
		}
	}

	return errors.Join(errs...)
}

type scopeKey struct {
	scope string
	key   string
}

// snapshot неизменяем после публикации в atomic.Pointer, поэтому читатели не блокируются.
type snapshot struct {
	revision int64
	configs  map[scopeKey]string
	flags    map[scopeKey]contracts.Flag
}

// Client держит локальный снапшот настроек и периодически обновляет его через gRPC.
// Пока сервис недоступен, чтение продолжает работать на последнем успешном снапшоте.
// До первой загрузки снапшот пуст: Value возвращает false, Enabled возвращает false,
// поэтому вызывающий код обязан иметь значения по умолчанию.
type Client struct {
	client onlineconfpb.OnlineConfServiceClient
	cfg    Config
	logger *slog.Logger

	snap     atomic.Pointer[snapshot]
	lastSync atomic.Int64 // unix nano последней успешной сверки с сервисом
}

var _ contracts.Reader = (*Client)(nil)

func New(conn grpc.ClientConnInterface, cfg Config, logger *slog.Logger) *Client {
	cfg.Scopes = uniqueScopes(cfg.Scopes)

	c := &Client{
		client: onlineconfpb.NewOnlineConfServiceClient(conn),
		cfg:    cfg,
		logger: logger,
	}
	c.snap.Store(&snapshot{})

	return c
}

// Run загружает снапшот и затем обновляет его каждые PollInterval до отмены ctx.
// Блокирует вызывающую горутину: запускается как `go client.Run(ctx)`.
// Неудача первой загрузки не фатальна, сервис стартует на значениях по умолчанию.
func (c *Client) Run(ctx context.Context) {
	c.refreshAndLog(ctx)

	ticker := time.NewTicker(c.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refreshAndLog(ctx)
		}
	}
}

// Refresh один раз запрашивает снапшот. При ошибке прежний снапшот сохраняется.
// Метод безопасен для параллельных вызовов, но нужен в основном тестам и стартовой
// синхронной загрузке.
func (c *Client) Refresh(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	current := c.snap.Load()

	resp, err := c.client.GetSnapshot(ctx, &onlineconfpb.GetSnapshotRequest{
		Scopes:        c.cfg.Scopes,
		KnownRevision: current.revision,
	})
	if err != nil {
		return c.wrapRPCError("get snapshot", err)
	}
	if resp == nil {
		return contracts.ErrEmptyResponse
	}

	c.lastSync.Store(time.Now().UnixNano())

	if resp.GetNotModified() {
		return nil
	}

	c.snap.Store(c.buildSnapshot(ctx, resp))

	return nil
}

func (c *Client) Value(scope, key string) (string, bool) {
	value, ok := c.snap.Load().configs[scopeKey{scope: scope, key: key}]
	return value, ok
}

func (c *Client) Enabled(scope, key string, userID uuid.UUID) bool {
	return c.Decide(scope, key, userID).Enabled
}

// Decide вычисляет флаг вместе с причиной результата.
func (c *Client) Decide(scope, key string, userID uuid.UUID) contracts.Decision {
	flag, ok := c.snap.Load().flags[scopeKey{scope: scope, key: key}]
	if !ok {
		return contracts.Decision{Enabled: false, Reason: contracts.ReasonFlagNotFound}
	}

	return Evaluate(flag, userID)
}

// Revision возвращает ревизию текущего снапшота; 0 — снапшот ещё не загружен.
func (c *Client) Revision() int64 {
	return c.snap.Load().revision
}

// LastSync возвращает время последней успешной сверки с сервисом; нулевое время — сверок не было.
func (c *Client) LastSync() time.Time {
	nanos := c.lastSync.Load()
	if nanos == 0 {
		return time.Time{}
	}

	return time.Unix(0, nanos)
}

func (c *Client) refreshAndLog(ctx context.Context) {
	if err := c.Refresh(ctx); err != nil && ctx.Err() == nil {
		c.logger.WarnContext(ctx, "onlineconf client: refresh failed, keeping previous snapshot",
			"error", err, "revision", c.Revision())
	}
}

func (c *Client) buildSnapshot(ctx context.Context, resp *onlineconfpb.GetSnapshotResponse) *snapshot {
	next := &snapshot{
		revision: resp.GetRevision(),
		configs:  make(map[scopeKey]string, len(resp.GetConfigs())),
		flags:    make(map[scopeKey]contracts.Flag, len(resp.GetFlags())),
	}

	for _, cfg := range resp.GetConfigs() {
		if cfg == nil {
			continue
		}
		next.configs[scopeKey{scope: cfg.GetScope(), key: cfg.GetKey()}] = cfg.GetValue()
	}

	for _, f := range resp.GetFlags() {
		if f == nil {
			continue
		}

		next.flags[scopeKey{scope: f.GetScope(), key: f.GetKey()}] = contracts.Flag{
			Scope:          f.GetScope(),
			Key:            f.GetKey(),
			RolloutPercent: f.GetRolloutPercent(),
			ForceOn:        c.parseUserIDs(ctx, f, f.GetForceOnUserIds()),
			ForceOff:       c.parseUserIDs(ctx, f, f.GetForceOffUserIds()),
		}
	}

	return next
}

func (c *Client) parseUserIDs(ctx context.Context, f *onlineconfpb.Flag, raw []string) map[uuid.UUID]struct{} {
	result := make(map[uuid.UUID]struct{}, len(raw))

	for _, value := range raw {
		id, err := uuid.Parse(value)
		if err != nil {
			c.logger.WarnContext(ctx, "onlineconf client: invalid user id in flag override",
				"scope", f.GetScope(), "flag", f.GetKey(), "user_id", value)
			continue
		}
		result[id] = struct{}{}
	}

	return result
}

func (c *Client) wrapRPCError(operation string, err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument:
		return fmt.Errorf("%w: %s: %v", contracts.ErrInvalidScopes, operation, err)
	case codes.Unavailable, codes.DeadlineExceeded:
		return fmt.Errorf("%w: %s: %v", contracts.ErrUnavailable, operation, err)
	default:
		return fmt.Errorf("onlineconf %s: %w", operation, err)
	}
}

func uniqueScopes(scopes []string) []string {
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

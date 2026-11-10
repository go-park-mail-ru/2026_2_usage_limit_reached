package testutils

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

func SetupValidator() *validator.Validator {
	v, err := validator.NewValidator(validator.DefaultConfig())
	if err != nil {
		panic(err)
	}
	return v
}

func SetupLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type staticVerifier json.RawMessage

func (v staticVerifier) Verify(string) (json.RawMessage, error) {
	return json.RawMessage(v), nil
}

func WithUser(r *http.Request, userID uuid.UUID) *http.Request {
	payload, err := json.Marshal(jwt.UserPayload{UserID: userID})
	if err != nil {
		panic(err)
	}
	r.AddCookie(&http.Cookie{Name: jwt.CookieName, Value: "test-token"})

	var authorized *http.Request
	capture := http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) { authorized = req })
	middleware.AuthMiddleware(staticVerifier(payload))(capture).ServeHTTP(httptest.NewRecorder(), r)
	return authorized
}

func DecodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var v T
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&v))
	return v
}

func SetupPgxMock(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()

	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pool.ExpectationsWereMet())
		pool.Close()
	})
	return pool
}

func SQLInOrder(fragments ...string) string {
	quoted := make([]string, len(fragments))
	for i, f := range fragments {
		quoted[i] = regexp.QuoteMeta(f)
	}
	return "(?s)" + strings.Join(quoted, ".*")
}

func PgError(code string) error {
	return &pgconn.PgError{Code: code}
}

func ExpectTxCommitted(pool pgxmock.PgxPoolIface) {
	pool.ExpectCommit()
	pool.ExpectRollback().WillReturnError(pgx.ErrTxClosed)
}

func ExpectTxRolledBack(pool pgxmock.PgxPoolIface) {
	pool.ExpectRollback()
	pool.ExpectRollback().WillReturnError(pgx.ErrTxClosed)
}

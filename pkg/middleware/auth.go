package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
)

var (
	ErrAuthUnavailable = errors.New("auth service unavailable")
	ErrInvalidToken    = errors.New("invalid token")
)

type TokenVerifier interface {
	Verify(ctx context.Context, token string) (json.RawMessage, error)
}

type ctxKey struct{}

func AuthMiddleware(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(jwt.CookieName)
			if err != nil {
				response.ErrorUnauthorized(w)
				return
			}

			raw, err := verifier.Verify(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, ErrAuthUnavailable) {
					response.ErrorServiceUnavailable(w)
					return
				}

				response.ErrorUnauthorized(w)
				return
			}

			var payload jwt.UserPayload
			if err := json.Unmarshal(raw, &payload); err != nil || payload.UserID == uuid.Nil {
				response.ErrorUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, &payload)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuthMiddleware(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(jwt.CookieName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			raw, err := verifier.Verify(r.Context(), cookie.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			var payload jwt.UserPayload
			if err := json.Unmarshal(raw, &payload); err != nil || payload.UserID == uuid.Nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, &payload)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserInfoFromContext(ctx context.Context) (*jwt.UserPayload, bool) {
	user, ok := ctx.Value(ctxKey{}).(*jwt.UserPayload)
	return user, ok
}

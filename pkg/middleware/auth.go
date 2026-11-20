package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

var ErrAuthUnavailable = errors.New("auth service unavailable")

type TokenVerifier interface {
	VerifyToken(ctx context.Context, token string) (*jwt.UserPayload, error)
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

			payload, err := verifier.VerifyToken(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, ErrAuthUnavailable) {
					response.ErrorServiceUnavailable(w)
					return
				}

				response.ErrorUnauthorized(w)
				return
			}

			if payload == nil {
				response.ErrorUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, payload)
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

			payload, err := verifier.VerifyToken(r.Context(), cookie.Value)
			if err != nil || payload == nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, payload)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserInfoFromContext(ctx context.Context) (*jwt.UserPayload, bool) {
	user, ok := ctx.Value(ctxKey{}).(*jwt.UserPayload)
	return user, ok
}

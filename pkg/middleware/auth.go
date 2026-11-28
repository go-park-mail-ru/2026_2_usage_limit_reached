package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/authn"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

type TokenVerifier interface {
	VerifyToken(ctx context.Context, token string) (*authn.User, error)
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

			user, err := verifier.VerifyToken(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, authn.ErrUnavailable) {
					response.ErrorServiceUnavailable(w)
					return
				}

				response.ErrorUnauthorized(w)
				return
			}

			if user == nil {
				response.ErrorUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminMiddleware requires a valid session with the admin role.
func AdminMiddleware(verifier TokenVerifier) func(http.Handler) http.Handler {
	authenticated := AuthMiddleware(verifier)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, ok := UserInfoFromContext(r.Context())
				if !ok {
					response.ErrorUnauthorized(w)
					return
				}
				if user.Role != "admin" {
					response.ErrorForbidden(w)
					return
				}

				next.ServeHTTP(w, r)
			})).ServeHTTP(w, r)
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

			user, err := verifier.VerifyToken(r.Context(), cookie.Value)
			if err != nil || user == nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserInfoFromContext(ctx context.Context) (*authn.User, bool) {
	user, ok := ctx.Value(ctxKey{}).(*authn.User)
	return user, ok
}

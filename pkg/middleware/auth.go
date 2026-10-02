package middleware

import (
	"context"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/token"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
)

type TokenVerifier interface {
	Verify(tokenString string) (*usecase.UserPayload, error)
}

type ctxKey struct{}

func AuthMiddleware(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(token.CookieName)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			payload, err := verifier.Verify(cookie.Value)
			if err != nil || payload == nil || payload.UserID == uuid.Nil {
				response.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, payload.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}

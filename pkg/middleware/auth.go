package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
)

type TokenVerifier interface {
	Verify(tokenString string) (json.RawMessage, error)
}

type ctxKey struct{}

func AuthMiddleware(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(jwt.CookieName)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			rawPayload, err := verifier.Verify(cookie.Value)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			var payload jwt.UserPayload
			if err := json.Unmarshal(rawPayload, &payload); err != nil {
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

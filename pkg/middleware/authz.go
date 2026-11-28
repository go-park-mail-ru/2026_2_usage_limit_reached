package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

// PermissionChecker проверяет актуальное право пользователя.
// Реализация — на стороне сервиса: OnlineConf делает живой SELECT роли
// и статуса из БД, поэтому отзыв прав действует на следующем запросе.
type PermissionChecker interface {
	CanManage(ctx context.Context, userID uuid.UUID) (bool, error)
}

// PermissionMiddleware пропускает дальше только пользователей с правом.
// Должен стоять после AuthMiddleware: без пользователя в контексте — 401.
func PermissionMiddleware(checker PermissionChecker, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserInfoFromContext(r.Context())
			if !ok || user == nil || user.ID == uuid.Nil {
				response.ErrorUnauthorized(w)
				return
			}

			allowed, err := checker.CanManage(r.Context(), user.ID)
			if err != nil {
				if logger != nil {
					logger.ErrorContext(r.Context(), "permission check failed",
						slog.String("error", err.Error()),
						slog.String("userID", user.ID.String()),
					)
				}
				response.ErrorInternal(w)
				return
			}

			if !allowed {
				response.ErrorForbidden(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NoStoreMiddleware запрещает кэширование ответов: права отзываемые,
// состояние динамическое — повторная проверка обязана видеть актуальные данные.
func NoStoreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

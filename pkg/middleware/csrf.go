package middleware

import (
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

const CSRFHeaderName = "X-CSRF-Token"

type CSRFVerifier interface {
	Binding(in csrf.BindingInput) (string, bool)
	Verify(binding, token string) bool
}

// Отклоняет изменяющие запросы без валидного CSRF-токена в заголовке X-CSRF-Token
func CSRFMiddleware(verifier CSRFVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get(CSRFHeaderName)
			binding, ok := verifier.Binding(CSRFInput(r))
			if !ok || !verifier.Verify(binding, token) {
				response.ErrorCSRFFailed(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func CSRFInput(r *http.Request) csrf.BindingInput {
	var in csrf.BindingInput

	if _, ok := UserInfoFromContext(r.Context()); ok {
		if cookie, err := r.Cookie(jwt.CookieName); err == nil {
			in.SessionToken = cookie.Value
		}
	}

	if cookie, err := r.Cookie(csrf.ContextCookieName); err == nil {
		in.AnonContext = cookie.Value
	}

	return in
}

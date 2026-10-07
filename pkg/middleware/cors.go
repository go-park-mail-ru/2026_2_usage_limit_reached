package middleware

import (
	"net/http"
)

type CORSConfig struct {
	AllowedOrigins map[string]struct{}
	AllowedMethods string
	AllowedHeaders string
}

func CORSMiddleware(cfg *CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if _, ok := cfg.AllowedOrigins[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", cfg.AllowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", cfg.AllowedHeaders)
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

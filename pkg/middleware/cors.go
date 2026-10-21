package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods string   `mapstructure:"allowed_methods"`
	AllowedHeaders string   `mapstructure:"allowed_headers"`
}

func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedHeaders: "Content-Type, Authorization",
		AllowedMethods: "GET, POST, PUT, DELETE, OPTIONS, PATCH",
	}
}

func (c CORSConfig) Validate() error {
	var errs []error
	if len(c.AllowedOrigins) == 0 {
		errs = append(errs, errors.New("cors: allowed_origins must not be empty"))
	}

	for _, origin := range c.AllowedOrigins {
		if strings.TrimSpace(origin) == "" {
			errs = append(errs, errors.New("cors: allowed_origins must not contain empty values"))
			break
		}
	}

	if strings.TrimSpace(c.AllowedMethods) == "" {
		errs = append(errs, errors.New("cors: allowed_methods must not be empty"))
	}
	if strings.TrimSpace(c.AllowedHeaders) == "" {
		errs = append(errs, errors.New("cors: allowed_headers must not be empty"))
	}

	return errors.Join(errs...)
}

func (c CORSConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("allowed_origins", c.AllowedOrigins),
		slog.String("allowed_methods", c.AllowedMethods),
		slog.String("allowed_headers", c.AllowedHeaders),
	)
}

func CORSMiddleware(cfg CORSConfig) func(http.Handler) http.Handler {
	allowedOrigins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowedOrigins[origin] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if _, ok := allowedOrigins[origin]; ok {
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

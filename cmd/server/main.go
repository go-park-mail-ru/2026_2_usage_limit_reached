package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery/handlers"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/token"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/middleware"
	profilehandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/delivery/handlers"
	profilememory "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/repository/memory"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	memory "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/repository/memory"
	"github.com/gorilla/mux"
)

func main() {
	secret := "secret-string"                   // должно читаться из .env через config
	tokenTTL := time.Duration(30 * time.Second) // должно читаться из .env через config
	addr := ":8080"                             // должно читаться из .env через config

	tokens := token.NewManager(secret, tokenTTL)

	repo := memory.NewUserRepository()
	profileRepo := profilememory.NewProfileRepository(nil, nil)
	uc := usecase.NewUsecase(repo, tokens)
	h := handlers.NewHandler(uc, tokens.TTL())

	auth := middleware.AuthMiddleware(tokens)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	profileHandler := profilehandlers.NewProfileHandler(
		profileusecase.NewProfileUsecase(repo, profileRepo),
		middleware.UserIDFromContext,
		slog.Default(),
	)
	profileHandler.RegisterRoutes(r, auth)

	srv := &http.Server{Addr: addr, Handler: r}
	fmt.Println("server running on :8080")
	log.Fatal(srv.ListenAndServe())
}

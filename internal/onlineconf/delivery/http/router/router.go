package router

import (
	"net/http"

	"github.com/gorilla/mux"

	onlineconfhttp "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/delivery/http"
)

type Handlers struct {
	Access  *onlineconfhttp.AccessHandler
	Scopes  *onlineconfhttp.ScopesHandler
	Configs *onlineconfhttp.ConfigsHandler
	Flags   *onlineconfhttp.FlagsHandler
	History *onlineconfhttp.HistoryHandler
	CSRF    *onlineconfhttp.CSRFHandler
}

// RegisterRoutes вешает маршруты OnlineConf: публичный /csrf — на public
// (OptionalAuth + CSRF), админский API — на private (Auth + CSRF +
// PermissionMiddleware + NoStore, добавляются в app при сборке саброутера).
//
// Префикса /api в путях нет — как и у остальных сервисов репо;
// /api срезает nginx на хосте фронтов.
func RegisterRoutes(public *mux.Router, private *mux.Router, deps Handlers) {
	public.HandleFunc("/csrf", deps.CSRF.GetToken).Methods(http.MethodGet, http.MethodOptions)

	private.HandleFunc("/onlineconf/admin/access", deps.Access.GetAccess).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/scopes", deps.Scopes.ListScopes).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/scopes", deps.Scopes.CreateScope).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/configs", deps.Configs.ListConfigs).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/configs", deps.Configs.CreateConfig).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/configs/{scope}/{key}", deps.Configs.GetConfig).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/configs/{scope}/{key}", deps.Configs.UpdateConfig).Methods(http.MethodPut, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/configs/{scope}/{key}", deps.Configs.DeleteConfig).Methods(http.MethodDelete, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/flags", deps.Flags.ListFlags).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/flags", deps.Flags.CreateFlag).Methods(http.MethodPost, http.MethodOptions)
	// literal-сегмент check регистрируется до {scope}/{key}
	private.HandleFunc("/onlineconf/admin/flags/check", deps.Flags.CheckFlag).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/flags/{scope}/{key}", deps.Flags.GetFlag).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/flags/{scope}/{key}", deps.Flags.DeleteFlag).Methods(http.MethodDelete, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/flags/{scope}/{key}/rollout", deps.Flags.UpdateRollout).Methods(http.MethodPut, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/flags/{scope}/{key}/force", deps.Flags.SetForce).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/flags/{scope}/{key}/force/{user_id}", deps.Flags.RemoveForce).Methods(http.MethodDelete, http.MethodOptions)
	private.HandleFunc("/onlineconf/admin/history", deps.History.ListHistory).Methods(http.MethodGet, http.MethodOptions)
}

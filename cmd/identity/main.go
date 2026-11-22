package main

import (
	"flag"
	"log"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/app"
	identityconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/config"
)

// @title Patreon Clone Identity API
// @version 1.0
// @description Аутентификация, профили и данные пользователей.
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name __Host-token
func main() {
	var configPath string
	flag.StringVar(&configPath, "config-file", identityconfig.DefaultConfigPath, "path to yaml config")
	flag.Parse()

	if err := app.Run(configPath); err != nil {
		log.Fatal(err)
	}
}

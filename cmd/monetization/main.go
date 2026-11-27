package main

import (
	"flag"
	"log"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/app"
	monetizationconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/config"
)

// @title Patreon Clone Monetization API
// @version 1.0
// @description API монетизации. Сейчас доступны создание донатов с оплатой и просмотр истории донатов.
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name __Host-token
func main() {
	var configPath string
	flag.StringVar(&configPath, "config-file", monetizationconfig.DefaultConfigPath, "path to yaml config")
	flag.Parse()

	if err := app.Run(configPath); err != nil {
		log.Fatal(err)
	}
}

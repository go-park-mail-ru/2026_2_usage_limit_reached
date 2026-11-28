package main

import (
	"flag"
	"log"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/app"
	onlineconfconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/onlineconf/config"
)

// @title Patreon Clone OnlineConf API
// @version 1.0
// @description Динамические конфигурации и feature flags: админ-API, /front и gRPC-срезы.
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name __Host-token
func main() {
	var configPath string
	flag.StringVar(&configPath, "config-file", onlineconfconfig.DefaultConfigPath, "path to yaml config")
	flag.Parse()

	if err := app.Run(configPath); err != nil {
		log.Fatal(err)
	}
}

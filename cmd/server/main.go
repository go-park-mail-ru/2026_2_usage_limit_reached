package main

import (
	"flag"
	"log"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/app"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
)

// @title Patreon Clone API
// @version 1.0
// @description REST API для платформы публикации контента (аналог Patreon).
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name __Host-token
func main() {
	var configPath string
	flag.StringVar(&configPath, "config-file", config.DefaultConfigPath, "path to yaml config")
	flag.Parse()

	if err := app.Run(configPath); err != nil {
		log.Fatal(err)
	}
}

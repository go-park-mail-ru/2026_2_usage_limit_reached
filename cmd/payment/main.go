package main

import (
	"flag"
	"log"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payment/app"
	paymentconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payment/config"
)

// @title Patreon Clone Payment API
// @version 1.0
// @description Платежи.
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name __Host-token
func main() {
	var configPath string
	flag.StringVar(&configPath, "config-file", paymentconfig.DefaultConfigPath, "path to yaml config")
	flag.Parse()

	if err := app.Run(configPath); err != nil {
		log.Fatal(err)
	}
}

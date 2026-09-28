package config

import "time"

type Config struct {
	Addr     string
	secret   string
	tokenTTL time.Duration
}

func Load() Config {
	// Чтение параметров из .env
	return Config{}
}

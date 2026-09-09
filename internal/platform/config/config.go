package config

import (
	"os"
)

type Config struct {
	HTTPHost string
	HTTPPort string
}

func Load() *Config {
	return &Config{
		HTTPHost: os.Getenv("HTTP_HOST"),
		HTTPPort: os.Getenv("HTTP_PORT"),
	}
}

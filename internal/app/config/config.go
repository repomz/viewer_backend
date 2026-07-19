package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config is a config :).
type Config struct {
	HTTPAddr       string
	DB_DSN         string
	MigrationsPath string
}

// Read reads config from environment.
func Read() Config {
	_ = godotenv.Load()

	config := Config{
		HTTPAddr:       strings.TrimSpace(os.Getenv("HTTP_ADDR")),
		DB_DSN:         strings.TrimSpace(os.Getenv("DB_DSN")),
		MigrationsPath: strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")),
	}
	if config.HTTPAddr == "" {
		config.HTTPAddr = ":8080"
	}
	return config
}

func (c Config) Validate() error {
	if c.DB_DSN == "" {
		return errors.New("DB_DSN is required")
	}
	return nil
}

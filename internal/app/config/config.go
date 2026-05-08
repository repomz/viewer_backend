package config

import (
	"os"

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
	var config Config

	godotenv.Load()

	httpAddr, exists := os.LookupEnv("HTTP_ADDR")
	if exists {
		config.HTTPAddr = httpAddr
	}
	// dsn, exists := os.LookupEnv("DB_DSN")
	// fmt.Println(dsn)
	// if exists {
	// 	config.DB_DSN = dsn
	// }
	config.DB_DSN = os.Getenv("DB_DSN")

	migrationsPath, exists := os.LookupEnv("MIGRATIONS_DIR")
	if exists {
		config.MigrationsPath = migrationsPath
	}
	return config
}

package config

import (
	"os"
	"strings"
)

type Config struct {
	Port      string
	BaseURL   string
	Database  string
	Debug     bool
	JWTSecret string
}

func Load() *Config {
	debug := strings.ToLower(os.Getenv("DEBUG")) == "true"

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "/"
	}
	if !strings.HasPrefix(baseURL, "/") {
		baseURL = "/" + baseURL
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL = baseURL + "/"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "capacitarr.db"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "development_secret_do_not_use_in_production"
	}

	return &Config{
		Port:      port,
		BaseURL:   baseURL,
		Database:  dbPath,
		Debug:     debug,
		JWTSecret: jwtSecret,
	}
}

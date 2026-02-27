package api

import (
	"net/http"

	"github.com/capacitarr/capacitarr/backend/internal/config"
)

func SetupRouter(cfg *config.Config) *http.ServeMux {
	mux := http.NewServeMux()

	prefix := cfg.BaseURL
	if prefix == "/" {
		prefix = "" // allow mapping directly to routes without double slashing
	}

	// Public routes
	mux.HandleFunc("GET " + prefix + "/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("POST " + prefix + "/api/v1/auth/login", LoginHandler(cfg))

	// Protected routes (Session-based)
	mux.HandleFunc("POST " + prefix + "/api/v1/auth/apikey", AuthMiddleware(cfg, GenerateAPIKeyHandler(cfg)))

	// Example protected programmatic route (API Key based)
	mux.HandleFunc("GET " + prefix + "/api/v1/data", APIKeyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success", "data": "protected hardware data"}`))
	}))

	return mux
}

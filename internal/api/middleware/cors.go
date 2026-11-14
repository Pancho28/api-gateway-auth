package middleware

import (
	"net/http"

	"api-gateway/internal/config"
	"github.com/go-chi/cors"
)

// SetupCORS configura el middleware de CORS soportando múltiples orígenes separados por coma
func SetupCORS(cfg *config.Config) func(http.Handler) http.Handler {
	origins := cfg.AllowedOrigins()
	allowCredentials := !(len(origins) == 1 && origins[0] == "*")

	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: allowCredentials,
		MaxAge:           300,
	})
}

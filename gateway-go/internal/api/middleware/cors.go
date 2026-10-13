package middleware

import (
	"net/http"

	"api-gateway/internal/config"
	"github.com/go-chi/cors"
)

// SetupCORS configura el middleware de CORS según las especificaciones del proyecto
func SetupCORS(cfg *config.Config) func(http.Handler) http.Handler {
	origins := []string{"*"}
	if cfg.Origin != "" {
		origins = []string{cfg.Origin}
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

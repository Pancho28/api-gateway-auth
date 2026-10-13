package api

import (
	"net/http"

	"api-gateway/internal/api/handlers"
	"api-gateway/internal/api/middleware"
	"api-gateway/internal/auth"
	"api-gateway/internal/config"
	"api-gateway/internal/database"
	"api-gateway/internal/proxy"
	"api-gateway/internal/redis"
	"api-gateway/internal/user"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-redis/redis_rate/v10"
)

// SetupRouter construye y configura el árbol de enrutamiento y middlewares de Chi
func SetupRouter(
	cfg *config.Config,
	db database.UserRepository,
	rdb *redis.Client,
) http.Handler {
	r := chi.NewRouter()

	// 1. Middlewares Globales
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetupCORS(cfg))
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)

	// Rate Limiter distribuido en Redis
	rateLimiter := middleware.NewRateLimiter(rdb)

	// Servicios y Handlers
	authService := auth.NewService(db, rdb, cfg)
	userService := user.NewService(db)
	proxyService := proxy.NewService(cfg, rdb)

	authHandler := handlers.NewAuthHandler(authService)
	userHandler := handlers.NewUserHandler(userService)
	gatewayHandler := handlers.NewGatewayHandler(proxyService)
	authMiddleware := middleware.NewAuthMiddleware(cfg, db, rdb)

	// 2. Rutas Públicas
	r.Group(func(public chi.Router) {
		public.With(rateLimiter.Limit(redis_rate.PerMinute(5))).Get("/health", gatewayHandler.HealthCheck)
		public.With(rateLimiter.Limit(redis_rate.PerMinute(10))).Post("/auth/login", authHandler.Login)
	})

	// 3. Rutas de Gestión de Usuarios (Requieren Rol ADMIN)
	r.Group(func(admin chi.Router) {
		admin.Use(authMiddleware.Authenticate)
		admin.Use(middleware.RequireAdmin)
		admin.With(rateLimiter.Limit(redis_rate.PerMinute(5))).Post("/users", userHandler.CreateUser)
	})

	// 4. Ruta Catch-All de Reverse Proxying (Requiere Autenticación)
	// Se registra al final para capturar todo el tráfico hacia los microservicios
	r.Group(func(proxied chi.Router) {
		proxied.Use(authMiddleware.Authenticate)
		proxied.Use(rateLimiter.Limit(redis_rate.PerMinute(20)))

		proxied.HandleFunc("/*", gatewayHandler.ReverseProxy)
	})

	return r
}

package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"api-gateway/internal/redis"
	"github.com/go-redis/redis_rate/v10"
)

type RateLimiter struct {
	limiter *redis_rate.Limiter
}

func NewRateLimiter(rdb *redis.Client) *RateLimiter {
	client := rdb.GetClient()
	if client == nil {
		return &RateLimiter{limiter: nil}
	}
	return &RateLimiter{
		limiter: redis_rate.NewLimiter(client),
	}
}

// Limit aplica rate limiting con clave basada en usuario (si está autenticado) o IP remota (Solución Vulnerabilidad 5)
func (rl *RateLimiter) Limit(rateLimit redis_rate.Limit) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rl.limiter == nil {
				// Degradación elegante si Redis no está activo
				next.ServeHTTP(w, r)
				return
			}

			// Clave determinada: prioridad Usuario autenticado en contexto, fallback a IP
			key := ""
			if user := GetUserFromContext(r.Context()); user != nil && user.Email != "" {
				key = fmt.Sprintf("rate_limit:user:%s", user.Email)
			} else {
				ip := getClientIP(r)
				key = fmt.Sprintf("rate_limit:ip:%s", ip)
			}

			res, err := rl.limiter.Allow(r.Context(), key, rateLimit)
			if err != nil {
				// Fallback elegante en fallo de Redis sin bloquear peticiones legítimas
				next.ServeHTTP(w, r)
				return
			}

			if res.Allowed == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"detail":"Rate limit exceeded: Demasiadas peticiones"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

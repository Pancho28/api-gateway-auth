package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/database"
	"api-gateway/internal/models"
	"api-gateway/internal/redis"
	"api-gateway/internal/security"
)

type contextKey string

const UserContextKey contextKey = "current_user"

type AuthMiddleware struct {
	cfg   *config.Config
	db    database.UserRepository
	redis *redis.Client
}

func NewAuthMiddleware(cfg *config.Config, db database.UserRepository, rdb *redis.Client) *AuthMiddleware {
	return &AuthMiddleware{
		cfg:   cfg,
		db:    db,
		redis: rdb,
	}
}

// Authenticate valida el token JWT e hidrata el usuario desde Redis (o PostgreSQL solo en Cache Miss)
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"detail":"No se pudieron validar las credenciales"}`, http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := security.ValidateToken(tokenStr, m.cfg.SecretKey)
		if err != nil {
			http.Error(w, `{"detail":"No se pudieron validar las credenciales"}`, http.StatusUnauthorized)
			return
		}

		email := claims.Subject

		// 1. Validación de sesión en Redis (ultra-baja latencia sin tocar PostgreSQL)
		var session models.SessionData
		sessionKey := fmt.Sprintf("session:%s", email)
		if m.redis.GetJSON(r.Context(), sessionKey, &session) && session.Email != "" {
			currentUser := &models.User{
				ID:    session.ID,
				Email: session.Email,
				Role:  session.Role,
			}
			ctx := context.WithValue(r.Context(), UserContextKey, currentUser)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// 2. Cache Miss: Consultar PostgreSQL solo si no está en Redis (Solución Vulnerabilidad 3)
		user, err := m.db.GetUserByEmail(r.Context(), email)
		if err != nil || user == nil {
			http.Error(w, `{"detail":"No se pudieron validar las credenciales"}`, http.StatusUnauthorized)
			return
		}

		// 3. Repoblar Redis (patrón Cache-Aside)
		expireDuration := time.Duration(m.cfg.AccessTokenExpireMinutes) * time.Minute
		m.redis.SetJSON(r.Context(), sessionKey, models.SessionData{
			ID:    user.ID,
			Email: user.Email,
			Role:  user.Role,
		}, expireDuration)

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin asegura que el usuario en el contexto tenga rol de ADMIN
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(UserContextKey).(*models.User)
		if !ok || user == nil || strings.ToUpper(user.Role) != "ADMIN" {
			http.Error(w, `{"detail":"No tiene los permisos necesarios"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetUserFromContext extrae el usuario autenticado del contexto si existe
func GetUserFromContext(ctx context.Context) *models.User {
	if u, ok := ctx.Value(UserContextKey).(*models.User); ok {
		return u
	}
	return nil
}

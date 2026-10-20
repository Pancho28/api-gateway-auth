package middleware

import (
	"context"
	"fmt"
	"log"
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
		if authHeader == "" {
			http.Error(w, `{"detail":"No se pudieron validar las credenciales [E-08]"}`, http.StatusUnauthorized)
			return
		}

		user, status, errResp := m.resolveUserFromAuthHeader(r.Context(), authHeader, r.Header.Get("X-Client-Id"))
		if errResp != "" {
			http.Error(w, errResp, status)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthenticateOptional valida el token JWT si está presente en la cabecera Authorization.
// Si no hay cabecera, permite que la petición continúe sin usuario en contexto.
func (m *AuthMiddleware) AuthenticateOptional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		user, status, errResp := m.resolveUserFromAuthHeader(r.Context(), authHeader, r.Header.Get("X-Client-Id"))
		if errResp != "" {
			http.Error(w, errResp, status)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveUserFromAuthHeader procesa la cabecera Authorization y resuelve el usuario autenticado
func (m *AuthMiddleware) resolveUserFromAuthHeader(ctx context.Context, authHeader, reqClientID string) (*models.User, int, string) {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		log.Printf("[AUTH] [TOKEN_REJECTED] Formato de autorización no es Bearer")
		return nil, http.StatusUnauthorized, `{"detail":"No se pudieron validar las credenciales [E-08]"}`
	}

	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := security.ValidateToken(tokenStr, m.cfg.SecretKey)
	if err != nil {
		log.Printf("[AUTH] [TOKEN_INVALID] Fallo al validar token Bearer: %v", err)
		return nil, http.StatusUnauthorized, `{"detail":"No se pudieron validar las credenciales [E-08]"}`
	}

	// Token Binding Check: si la petición incluye X-Client-Id, debe coincidir estrictamente con el token
	reqClientID = strings.TrimSpace(reqClientID)
	if reqClientID != "" && claims.ClientID != "" && reqClientID != claims.ClientID {
		log.Printf("[SECURITY] [TOKEN_BINDING_MISMATCH] Token ClientID ('%s') != Request ClientID ('%s')", claims.ClientID, reqClientID)
		return nil, http.StatusUnauthorized, `{"detail":"Contexto de seguridad inconsistente [E-02]"}`
	}

	email := claims.Subject

	// 1. Validación de sesión en Redis (ultra-baja latencia sin tocar PostgreSQL)
	var session models.SessionData
	sessionKey := fmt.Sprintf("session:%s", email)
	if m.redis.GetJSON(ctx, sessionKey, &session) && session.Email != "" {
		currentUser := &models.User{
			ID:       session.ID,
			Email:    session.Email,
			Role:     session.Role,
			ClientID: claims.ClientID,
			Scopes:   session.Scopes,
		}
		if currentUser.ClientID == "" {
			currentUser.ClientID = session.ClientID
		}
		if len(currentUser.Scopes) == 0 {
			currentUser.Scopes = claims.Scopes
		}
		return currentUser, 0, ""
	}

	// 2. Cache Miss: Consultar PostgreSQL solo si no está en Redis (Solución Vulnerabilidad 3)
	user, err := m.db.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, http.StatusUnauthorized, `{"detail":"No se pudieron validar las credenciales [E-08]"}`
	}
	user.ClientID = claims.ClientID
	user.Scopes = claims.Scopes

	// 3. Repoblar Redis (patrón Cache-Aside)
	expireDuration := time.Duration(m.cfg.AccessTokenExpireMinutes) * time.Minute
	m.redis.SetJSON(ctx, sessionKey, models.SessionData{
		ID:       user.ID,
		Email:    user.Email,
		Role:     user.Role,
		ClientID: user.ClientID,
		Scopes:   user.Scopes,
	}, expireDuration)

	return user, 0, ""
}

// RequireAdmin asegura que el usuario en el contexto tenga rol de admin
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(UserContextKey).(*models.User)
		if !ok || user == nil {
			http.Error(w, `{"detail":"Permisos insuficientes para el recurso [E-09]"}`, http.StatusForbidden)
			return
		}
		role := strings.ToLower(strings.TrimSpace(user.Role))
		if role != models.RoleAdmin {
			http.Error(w, `{"detail":"Permisos insuficientes para el recurso [E-09]"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireGatewaySecret verifica que la petición entrante incluya un secreto compartido válido
// a través de la cabecera X-Gateway-Secret o Authorization: Bearer <secret>
func RequireGatewaySecret(gatewaySecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			providedSecret := strings.TrimSpace(r.Header.Get("X-Gateway-Secret"))
			if providedSecret == "" {
				authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
				if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
					providedSecret = strings.TrimSpace(authHeader[7:])
				}
			}

			if providedSecret == "" || gatewaySecret == "" || providedSecret != gatewaySecret {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"detail":"No se pudieron validar las credenciales [E-08]"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUserFromContext extrae el usuario autenticado del contexto si existe
func GetUserFromContext(ctx context.Context) *models.User {
	if u, ok := ctx.Value(UserContextKey).(*models.User); ok {
		return u
	}
	return nil
}

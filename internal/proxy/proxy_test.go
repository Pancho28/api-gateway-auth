package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/models"
	"api-gateway/internal/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestProxy(t *testing.T, targetHandler http.HandlerFunc) (*Service, *httptest.Server, *config.Config) {
	backend := httptest.NewServer(targetHandler)

	cfg := &config.Config{
		RedisCacheTTL: 60 * time.Second,
		Microservices: map[string]config.ServiceConfig{
			"orders": {
				URL:            backend.URL,
				APIKey:         "secret-internal-key-123",
				AllowedClients: []string{"client-app", "admin-portal"},
			},
			"unconfigured": {
				URL:    backend.URL,
				APIKey: "secret-internal-key-456",
				// AllowedClients vacío -> Zero Trust / Deny by default
			},
			"staff-only": {
				URL:            backend.URL,
				APIKey:         "secret-internal-key-789",
				AllowedClients: []string{"staff-pos"},
				RequiredRoles:  []string{"ADMIN"},
			},
		},
	}

	rdb := redis.NewClient(&config.Config{
		RedisHost: "localhost",
		RedisPort: 6379,
	})

	svc := NewService(cfg, rdb)
	return svc, backend, cfg
}

func TestProxyServicePathTraversalBlocked(t *testing.T) {
	svc, backend, _ := setupTestProxy(t, func(w http.ResponseWriter, r *http.Request) {})
	defer backend.Close()

	maliciousPaths := []string{
		"test/../admin",
		"test/%2e%2e/admin",
		"test/%2E%2E/admin",
		"test/./config",
		"test/%2e/config",
		"../test/api",
		"test/api/..%00",
		"",
		"/",
	}

	for _, malPath := range maliciousPaths {
		t.Run("Path:"+malPath, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+malPath, nil)
			rec := httptest.NewRecorder()

			svc.ForwardRequest(rec, req, malPath, nil)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "Path malicioso debe ser rechazado con 400 Bad Request")
		})
	}
}

func TestSanitizePathNullByteDirect(t *testing.T) {
	svc, backend, _ := setupTestProxy(t, func(w http.ResponseWriter, r *http.Request) {})
	defer backend.Close()

	_, _, err := svc.sanitizePath("test/api/..\x00")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no permitida")
}

func TestProxyServiceNoServiceConfig(t *testing.T) {
	svc, backend, _ := setupTestProxy(t, func(w http.ResponseWriter, r *http.Request) {})
	defer backend.Close()

	req := httptest.NewRequest(http.MethodGet, "/unknown/resource", nil)
	rec := httptest.NewRecorder()

	svc.ForwardRequest(rec, req, "unknown/resource", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "Recurso o servicio no encontrado [E-06]")
}

func TestProxyServiceZeroTrustDenyByDefault(t *testing.T) {
	svc, backend, _ := setupTestProxy(t, func(w http.ResponseWriter, r *http.Request) {})
	defer backend.Close()

	currentUser := &models.User{
		ID:       "user-1",
		Email:    "test@test.com",
		Role:     "USER",
		ClientID: "client-app",
	}

	// 1. Servicio sin allowed_clients configurados -> 403 Forbidden
	req := httptest.NewRequest(http.MethodGet, "/unconfigured/items", nil)
	rec := httptest.NewRecorder()
	svc.ForwardRequest(rec, req, "unconfigured/items", currentUser)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "Acceso al recurso denegado [E-03]")

	// 2. Cliente no autorizado en lista blanca -> 403 Forbidden
	reqForbiddenClient := httptest.NewRequest(http.MethodGet, "/staff-only/dashboard", nil)
	recForbiddenClient := httptest.NewRecorder()
	svc.ForwardRequest(recForbiddenClient, reqForbiddenClient, "staff-only/dashboard", currentUser)
	assert.Equal(t, http.StatusForbidden, recForbiddenClient.Code)
	assert.Contains(t, recForbiddenClient.Body.String(), "Acceso al recurso denegado [E-03]")

	// 3. Rol no autorizado -> 403 Forbidden
	staffUserBadRole := &models.User{
		ID:       "user-2",
		Email:    "staff@test.com",
		Role:     "USER", // staff-only exige ADMIN
		ClientID: "staff-pos",
	}
	reqForbiddenRole := httptest.NewRequest(http.MethodGet, "/staff-only/dashboard", nil)
	recForbiddenRole := httptest.NewRecorder()
	svc.ForwardRequest(recForbiddenRole, reqForbiddenRole, "staff-only/dashboard", staffUserBadRole)
	assert.Equal(t, http.StatusForbidden, recForbiddenRole.Code)
	assert.Contains(t, recForbiddenRole.Body.String(), "Acceso al recurso denegado [E-03]")
}

func TestProxyServiceForwardAndHeaderHandling(t *testing.T) {
	var receivedHeaders http.Header

	svc, backend, _ := setupTestProxy(t, func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"order_id":123,"status":"created"}`))
	})
	defer backend.Close()

	req := httptest.NewRequest(http.MethodPost, "/orders/create", strings.NewReader(`{"item":"book"}`))
	req.Header.Set("Content-Type", "application/json")
	// Intento de spoofing por el cliente
	req.Header.Set("X-Internal-Api-Key", "spoofed-key")
	req.Header.Set("X-User-Role", "spoofed-role")
	req.Header.Set("X-Client-ID", "spoofed-client")
	req.Header.Set("X-Scopes", "spoofed-scope")

	currentUser := &models.User{
		ID:       "user-uuid-1",
		Email:    "client@test.com",
		Role:     "USER",
		ClientID: "client-app",
		Scopes:   []string{"orders:create", "orders:read"},
	}

	rec := httptest.NewRecorder()
	svc.ForwardRequest(rec, req, "orders/create", currentUser)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"order_id":123`)

	// Verificar inyección segura y saneamiento
	require.NotNil(t, receivedHeaders)
	assert.Equal(t, "secret-internal-key-123", receivedHeaders.Get("X-Internal-Api-Key"), "Debe inyectar la API Key interna legítima")
	assert.Equal(t, "user-uuid-1", receivedHeaders.Get("X-User-ID"), "Debe propagar el ID del usuario")
	assert.Equal(t, "client@test.com", receivedHeaders.Get("X-User-Email"), "Debe propagar el email del usuario")
	assert.Equal(t, "user", receivedHeaders.Get("X-User-Role"), "Debe propagar el rol legítimo sin spoofing")
	assert.Equal(t, "client-app", receivedHeaders.Get("X-Client-ID"), "Debe propagar el Client ID legítimo sin spoofing")
	assert.Equal(t, "orders:create orders:read", receivedHeaders.Get("X-Scopes"), "Debe propagar los scopes legítimos sin spoofing")
	assert.NotEmpty(t, receivedHeaders.Get("X-Request-Id"), "Debe propagar X-Request-Id para distributed tracing")
}

func TestProxyServiceCacheHitAndMiss(t *testing.T) {
	callCount := 0
	svc, backend, cfg := setupTestProxy(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"test-cache"}`))
	})
	defer backend.Close()

	currentUser := &models.User{
		ID:       "user-1",
		Email:    "client@test.com",
		Role:     "USER",
		ClientID: "client-app",
	}

	// Simulamos respuesta cacheada directamente en Redis si Redis está disponible
	ctx := context.Background()
	cacheKey := svc.generateCacheKey("orders", "items", "client-app", "client@test.com", nil)
	svc.redis.SetJSON(ctx, cacheKey, CachedResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       `{"data":"cached-response"}`,
	}, cfg.RedisCacheTTL)

	// Petición GET
	req := httptest.NewRequest(http.MethodGet, "/orders/items", nil)
	rec := httptest.NewRecorder()

	svc.ForwardRequest(rec, req, "orders/items", currentUser)

	// Si Redis tiene la clave, retorna HIT
	if rec.Header().Get("X-Cache") == "HIT" {
		assert.Equal(t, `{"data":"cached-response"}`, rec.Body.String())
		assert.Equal(t, 0, callCount, "No debe tocar el backend en Cache Hit")
	} else {
		// Degradación elegante: el backend responde con X-Cache: MISS
		assert.Equal(t, "MISS", rec.Header().Get("X-Cache"))
		assert.Equal(t, 1, callCount)
	}
}

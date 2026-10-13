package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"api-gateway/internal/api"
	"api-gateway/internal/config"
	"api-gateway/internal/models"
	"api-gateway/internal/redis"
	"api-gateway/internal/security"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockUserRepository implementa database.UserRepository en memoria para tests aislados
type MockUserRepository struct {
	mu    sync.RWMutex
	users map[string]*models.User
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users: make(map[string]*models.User),
	}
}

func (m *MockUserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *MockUserRepository) CreateUser(ctx context.Context, user *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.Email] = user
	return nil
}

// setupTestEnvironment inicializa un router Chi completo con mock DB y mock backend
func setupTestEnvironment(t *testing.T) (http.Handler, *MockUserRepository, *config.Config, *httptest.Server) {
	// 1. Backend microservicio simulado
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/items" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":1,"name":"Item A"}]`))
			return
		}
		if r.URL.Path == "/orders" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"order_id":"ord-123","status":"approved"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	cfg := &config.Config{
		SecretKey:                "test-very-secure-secret-key-123456",
		AccessTokenExpireMinutes: 30,
		RedisCacheTTL:            60 * time.Second,
		Microservices: map[string]config.ServiceConfig{
			"shop": {
				URL:    backend.URL,
				APIKey: "backend-secret-api-key",
			},
		},
	}

	mockDB := NewMockUserRepository()

	// Seed de admin de prueba
	adminHash, err := security.HashPassword("Admin1234")
	require.NoError(t, err)
	mockDB.users["admin@test.com"] = &models.User{
		ID:             uuid.New().String(),
		Email:          "admin@test.com",
		HashedPassword: adminHash,
		Role:           "ADMIN",
	}

	// Seed de usuario regular
	userHash, err := security.HashPassword("UserPass1")
	require.NoError(t, err)
	mockDB.users["user@test.com"] = &models.User{
		ID:             uuid.New().String(),
		Email:          "user@test.com",
		HashedPassword: userHash,
		Role:           "USER",
	}

	rdb := redis.NewClient(&config.Config{
		RedisHost: "localhost",
		RedisPort: 6379,
	})

	router := api.SetupRouter(cfg, mockDB, rdb)
	return router, mockDB, cfg, backend
}

func TestHealthCheckEndpoint(t *testing.T) {
	router, _, _, backend := setupTestEnvironment(t)
	defer backend.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"healthy"`)
	assert.Contains(t, rec.Body.String(), `"gateway_version":"1.0.0"`)
}

func TestAuthLoginEndpoint(t *testing.T) {
	router, _, _, backend := setupTestEnvironment(t)
	defer backend.Close()

	// 1. Login exitoso con credenciales válidas
	body, _ := json.Marshal(models.UserLoginRequest{
		Email:    "admin@test.com",
		Password: "Admin1234",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var tokenResp models.TokenResponse
	err := json.Unmarshal(rec.Body.Bytes(), &tokenResp)
	require.NoError(t, err)
	assert.Equal(t, "admin@test.com", tokenResp.User)
	assert.Equal(t, "ADMIN", tokenResp.Role)
	assert.NotEmpty(t, tokenResp.Token)

	// 2. Login con contraseña incorrecta
	badPassBody, _ := json.Marshal(models.UserLoginRequest{
		Email:    "admin@test.com",
		Password: "WrongPassword123",
	})
	reqBad := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(badPassBody))
	recBad := httptest.NewRecorder()
	router.ServeHTTP(recBad, reqBad)

	assert.Equal(t, http.StatusUnauthorized, recBad.Code)
	assert.Equal(t, "Bearer", recBad.Header().Get("WWW-Authenticate"))
	assert.Contains(t, recBad.Body.String(), "Correo o contraseña incorrectos")

	// 3. Login con usuario inexistente
	notFoundBody, _ := json.Marshal(models.UserLoginRequest{
		Email:    "nonexistent@test.com",
		Password: "Admin1234",
	})
	reqNotFound := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(notFoundBody))
	recNotFound := httptest.NewRecorder()
	router.ServeHTTP(recNotFound, reqNotFound)

	assert.Equal(t, http.StatusUnauthorized, recNotFound.Code)
}

func TestUsersEndpointPermissions(t *testing.T) {
	router, _, cfg, backend := setupTestEnvironment(t)
	defer backend.Close()

	newUserBody, _ := json.Marshal(models.AdminUserCreateRequest{
		Email:    "created@test.com",
		Password: "NewPassword1",
		Role:     "USER",
	})

	// 1. Petición sin autenticar -> 401
	reqUnauth := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(newUserBody))
	recUnauth := httptest.NewRecorder()
	router.ServeHTTP(recUnauth, reqUnauth)
	assert.Equal(t, http.StatusUnauthorized, recUnauth.Code)

	// 2. Petición autenticada como usuario regular (USER) -> 403 Forbidden
	userToken, err := security.GenerateToken("user@test.com", "USER", cfg.SecretKey, 15*time.Minute)
	require.NoError(t, err)

	reqForbidden := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(newUserBody))
	reqForbidden.Header.Set("Authorization", "Bearer "+userToken)
	recForbidden := httptest.NewRecorder()
	router.ServeHTTP(recForbidden, reqForbidden)
	assert.Equal(t, http.StatusForbidden, recForbidden.Code)
	assert.Contains(t, recForbidden.Body.String(), "No tiene los permisos necesarios")

	// 3. Petición autenticada como ADMIN -> 201 Created
	adminToken, err := security.GenerateToken("admin@test.com", "ADMIN", cfg.SecretKey, 15*time.Minute)
	require.NoError(t, err)

	reqAdmin := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(newUserBody))
	reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
	recAdmin := httptest.NewRecorder()
	router.ServeHTTP(recAdmin, reqAdmin)
	assert.Equal(t, http.StatusCreated, recAdmin.Code)

	var userResp models.UserResponse
	err = json.Unmarshal(recAdmin.Body.Bytes(), &userResp)
	require.NoError(t, err)
	assert.Equal(t, "created@test.com", userResp.Email)
	assert.Equal(t, "USER", userResp.Role)
	assert.NotEmpty(t, userResp.ID)

	// 4. Intento de duplicar email -> 400 Bad Request
	reqDup := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(newUserBody))
	reqDup.Header.Set("Authorization", "Bearer "+adminToken)
	recDup := httptest.NewRecorder()
	router.ServeHTTP(recDup, reqDup)
	assert.Equal(t, http.StatusBadRequest, recDup.Code)
	assert.Contains(t, recDup.Body.String(), "ya está registrado")
}

func TestReverseProxyForwardingAndHeaders(t *testing.T) {
	router, _, cfg, backend := setupTestEnvironment(t)
	defer backend.Close()

	// 1. Petición sin token al proxy -> 401 Unauthorized
	reqUnauth := httptest.NewRequest(http.MethodGet, "/shop/items", nil)
	recUnauth := httptest.NewRecorder()
	router.ServeHTTP(recUnauth, reqUnauth)
	assert.Equal(t, http.StatusUnauthorized, recUnauth.Code)

	// 2. Petición con token válido -> 200 OK y reenvío exitoso
	userToken, err := security.GenerateToken("user@test.com", "USER", cfg.SecretKey, 15*time.Minute)
	require.NoError(t, err)

	reqAuth := httptest.NewRequest(http.MethodGet, "/shop/items", nil)
	reqAuth.Header.Set("Authorization", "Bearer "+userToken)
	recAuth := httptest.NewRecorder()
	router.ServeHTTP(recAuth, reqAuth)

	assert.Equal(t, http.StatusOK, recAuth.Code)
	assert.Contains(t, recAuth.Body.String(), "Item A")

	// 3. Petición POST con streaming de cuerpo hacia microservicio
	orderBody := []byte(`{"product_id":"prod-1","qty":2}`)
	reqPost := httptest.NewRequest(http.MethodPost, "/shop/orders", bytes.NewReader(orderBody))
	reqPost.Header.Set("Authorization", "Bearer "+userToken)
	reqPost.Header.Set("Content-Type", "application/json")
	recPost := httptest.NewRecorder()
	router.ServeHTTP(recPost, reqPost)

	assert.Equal(t, http.StatusOK, recPost.Code)
	assert.Contains(t, recPost.Body.String(), "ord-123")

	// 4. Intento de Path Traversal bloqueado por el proxy
	reqTraversal := httptest.NewRequest(http.MethodGet, "/shop/../private", nil)
	reqTraversal.Header.Set("Authorization", "Bearer "+userToken)
	recTraversal := httptest.NewRecorder()
	router.ServeHTTP(recTraversal, reqTraversal)
	assert.Equal(t, http.StatusBadRequest, recTraversal.Code)
}

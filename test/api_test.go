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

var (
	lastProxyHeaders  http.Header
	proxyHeadersMutex sync.Mutex
)

func setLastProxyHeaders(h http.Header) {
	proxyHeadersMutex.Lock()
	defer proxyHeadersMutex.Unlock()
	lastProxyHeaders = h.Clone()
}

func getLastProxyHeaders() http.Header {
	proxyHeadersMutex.Lock()
	defer proxyHeadersMutex.Unlock()
	if lastProxyHeaders == nil {
		return nil
	}
	return lastProxyHeaders.Clone()
}

// setupTestEnvironment inicializa un router Chi completo con mock DB y mock backend
func setupTestEnvironment(t *testing.T) (http.Handler, *MockUserRepository, *config.Config, *httptest.Server) {
	// 1. Backend microservicio simulado
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setLastProxyHeaders(r.Header)
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
				URL:            backend.URL,
				APIKey:         "backend-secret-api-key",
				AllowedClients: []string{"test-client", "admin-client"},
			},
			"catalog": {
				URL:            backend.URL,
				APIKey:         "catalog-secret-api-key",
				AllowedClients: []string{"*"},
				PublicPaths:    []string{"/items"},
			},
			"unconfigured": {
				URL:    backend.URL,
				APIKey: "backend-secret-api-key",
				// AllowedClients vacío -> Zero Trust bloquea
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

	// 1. Intento de login SIN Client ID -> 400 Bad Request (Requerimiento estricto)
	bodyNoClient, _ := json.Marshal(models.UserLoginRequest{
		Email:    "admin@test.com",
		Password: "Admin1234",
	})
	reqNoClient := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(bodyNoClient))
	reqNoClient.Header.Set("Content-Type", "application/json")
	recNoClient := httptest.NewRecorder()
	router.ServeHTTP(recNoClient, reqNoClient)

	assert.Equal(t, http.StatusBadRequest, recNoClient.Code)
	assert.Contains(t, recNoClient.Body.String(), "Cabeceras requeridas ausentes o no válidas")
	assert.Contains(t, recNoClient.Body.String(), "E-01")

	// 2. Login exitoso con cabecera X-Client-Id
	bodyWithHeader, _ := json.Marshal(models.UserLoginRequest{
		Email:    "admin@test.com",
		Password: "Admin1234",
	})
	reqHeader := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(bodyWithHeader))
	reqHeader.Header.Set("Content-Type", "application/json")
	reqHeader.Header.Set("X-Client-Id", "admin-client")
	recHeader := httptest.NewRecorder()
	router.ServeHTTP(recHeader, reqHeader)

	assert.Equal(t, http.StatusOK, recHeader.Code)
	var tokenResp models.TokenResponse
	err := json.Unmarshal(recHeader.Body.Bytes(), &tokenResp)
	require.NoError(t, err)
	assert.Equal(t, "admin@test.com", tokenResp.User)
	assert.Equal(t, "ADMIN", tokenResp.Role)
	assert.Equal(t, "admin-client", tokenResp.ClientID)
	assert.NotEmpty(t, tokenResp.Token)

	// 3. Login exitoso con client_id en body JSON
	bodyWithField, _ := json.Marshal(models.UserLoginRequest{
		Email:    "admin@test.com",
		Password: "Admin1234",
		ClientID: "body-client",
	})
	reqBodyField := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(bodyWithField))
	reqBodyField.Header.Set("Content-Type", "application/json")
	recBodyField := httptest.NewRecorder()
	router.ServeHTTP(recBodyField, reqBodyField)

	assert.Equal(t, http.StatusOK, recBodyField.Code)
	var bodyResp models.TokenResponse
	err = json.Unmarshal(recBodyField.Body.Bytes(), &bodyResp)
	require.NoError(t, err)
	assert.Equal(t, "body-client", bodyResp.ClientID)

	// 4. Login con contraseña incorrecta
	badPassBody, _ := json.Marshal(models.UserLoginRequest{
		Email:    "admin@test.com",
		Password: "WrongPassword123",
	})
	reqBad := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(badPassBody))
	reqBad.Header.Set("Content-Type", "application/json")
	reqBad.Header.Set("X-Client-Id", "admin-client")
	recBad := httptest.NewRecorder()
	router.ServeHTTP(recBad, reqBad)

	assert.Equal(t, http.StatusUnauthorized, recBad.Code)
	assert.Equal(t, "Bearer", recBad.Header().Get("WWW-Authenticate"))
	assert.Contains(t, recBad.Body.String(), "No se pudieron validar las credenciales [E-08]")

	// 5. Login con usuario inexistente
	notFoundBody, _ := json.Marshal(models.UserLoginRequest{
		Email:    "nonexistent@test.com",
		Password: "Admin1234",
	})
	reqNotFound := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(notFoundBody))
	reqNotFound.Header.Set("Content-Type", "application/json")
	reqNotFound.Header.Set("X-Client-Id", "admin-client")
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
	userToken, err := security.GenerateToken("user@test.com", "USER", "test-client", cfg.SecretKey, 15*time.Minute)
	require.NoError(t, err)

	reqForbidden := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(newUserBody))
	reqForbidden.Header.Set("Authorization", "Bearer "+userToken)
	recForbidden := httptest.NewRecorder()
	router.ServeHTTP(recForbidden, reqForbidden)
	assert.Equal(t, http.StatusForbidden, recForbidden.Code)
	assert.Contains(t, recForbidden.Body.String(), "Permisos insuficientes para el recurso [E-09]")

	// 3. Petición autenticada como ADMIN -> 201 Created
	adminToken, err := security.GenerateToken("admin@test.com", "ADMIN", "admin-client", cfg.SecretKey, 15*time.Minute)
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

	// 4. Intento de duplicar email -> 409 Conflict [E-12]
	reqDup := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(newUserBody))
	reqDup.Header.Set("Authorization", "Bearer "+adminToken)
	recDup := httptest.NewRecorder()
	router.ServeHTTP(recDup, reqDup)
	assert.Equal(t, http.StatusConflict, recDup.Code)
	assert.Contains(t, recDup.Body.String(), "Conflicto con el recurso solicitado [E-12]")
}

func TestReverseProxyForwardingAndHeaders(t *testing.T) {
	router, _, cfg, backend := setupTestEnvironment(t)
	defer backend.Close()

	// 1. Petición sin token al proxy -> 401 Unauthorized
	reqUnauth := httptest.NewRequest(http.MethodGet, "/shop/items", nil)
	recUnauth := httptest.NewRecorder()
	router.ServeHTTP(recUnauth, reqUnauth)
	assert.Equal(t, http.StatusUnauthorized, recUnauth.Code)

	// 2. Petición con token válido y cliente autorizado -> 200 OK y reenvío exitoso
	userToken, err := security.GenerateToken("user@test.com", "USER", "test-client", cfg.SecretKey, 15*time.Minute)
	require.NoError(t, err)

	reqAuth := httptest.NewRequest(http.MethodGet, "/shop/items", nil)
	reqAuth.Header.Set("Authorization", "Bearer "+userToken)
	reqAuth.Header.Set("X-Client-Id", "test-client")
	// Intento de spoofing de API Key interna por el cliente
	reqAuth.Header.Set("X-Internal-Api-Key", "spoofed-key")
	recAuth := httptest.NewRecorder()
	router.ServeHTTP(recAuth, reqAuth)

	assert.Equal(t, http.StatusOK, recAuth.Code)
	assert.Contains(t, recAuth.Body.String(), "Item A")

	// Verificar inyección fidedigna y anti-spoofing en el backend
	headers := getLastProxyHeaders()
	require.NotNil(t, headers)
	assert.Equal(t, "test-client", headers.Get("X-Client-ID"), "Debe inyectar el client ID verificado del token")
	assert.Equal(t, "user@test.com", headers.Get("X-User-Email"))
	assert.Equal(t, "USER", headers.Get("X-User-Role"))
	assert.Equal(t, "backend-secret-api-key", headers.Get("X-Internal-Api-Key"))

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

	// 5. Zero Trust: Servicio sin AllowedClients configurados -> 403 Forbidden
	reqZeroTrust := httptest.NewRequest(http.MethodGet, "/unconfigured/data", nil)
	reqZeroTrust.Header.Set("Authorization", "Bearer "+userToken)
	recZeroTrust := httptest.NewRecorder()
	router.ServeHTTP(recZeroTrust, reqZeroTrust)
	assert.Equal(t, http.StatusForbidden, recZeroTrust.Code)
	assert.Contains(t, recZeroTrust.Body.String(), "Acceso al recurso denegado [E-03]")

	// 6. Particionamiento: Cliente no autorizado en lista blanca -> 403 Forbidden
	unauthorizedClientToken, err := security.GenerateToken("user@test.com", "USER", "unauthorized-app", cfg.SecretKey, 15*time.Minute)
	require.NoError(t, err)

	reqPartitionDenied := httptest.NewRequest(http.MethodGet, "/shop/items", nil)
	reqPartitionDenied.Header.Set("Authorization", "Bearer "+unauthorizedClientToken)
	recPartitionDenied := httptest.NewRecorder()
	router.ServeHTTP(recPartitionDenied, reqPartitionDenied)
	assert.Equal(t, http.StatusForbidden, recPartitionDenied.Code)
	assert.Contains(t, recPartitionDenied.Body.String(), "Acceso al recurso denegado [E-03]")

	// 7. Token Binding Mismatch: Cabecera X-Client-Id no coincide con el token -> 401 Unauthorized
	reqBindingMismatch := httptest.NewRequest(http.MethodGet, "/shop/items", nil)
	reqBindingMismatch.Header.Set("Authorization", "Bearer "+userToken) // token tiene test-client
	reqBindingMismatch.Header.Set("X-Client-Id", "imposter-client")     // cabecera dice imposter-client
	recBindingMismatch := httptest.NewRecorder()
	router.ServeHTTP(recBindingMismatch, reqBindingMismatch)
	assert.Equal(t, http.StatusUnauthorized, recBindingMismatch.Code)
	assert.Contains(t, recBindingMismatch.Body.String(), "Contexto de seguridad inconsistente [E-02]")
}

func TestAuthRegisterEndpoint(t *testing.T) {
	router, _, _, backend := setupTestEnvironment(t)
	defer backend.Close()

	// 1. Registro sin Client-ID -> 400 Bad Request [E-01]
	bodyNoClient, _ := json.Marshal(models.UserRegisterRequest{
		Email:    "newguest@test.com",
		Password: "Password123",
	})
	reqNoClient := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(bodyNoClient))
	reqNoClient.Header.Set("Content-Type", "application/json")
	recNoClient := httptest.NewRecorder()
	router.ServeHTTP(recNoClient, reqNoClient)
	assert.Equal(t, http.StatusBadRequest, recNoClient.Code)
	assert.Contains(t, recNoClient.Body.String(), "E-01")

	// 2. Registro con intento de rol ADMIN en registro público -> 400 Bad Request [E-11]
	bodyEscalation, _ := json.Marshal(models.UserRegisterRequest{
		Email:    "hacker@test.com",
		Password: "Password123",
		ClientID: "web-client",
		Role:     "ADMIN",
	})
	reqEscalation := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(bodyEscalation))
	reqEscalation.Header.Set("Content-Type", "application/json")
	recEscalation := httptest.NewRecorder()
	router.ServeHTTP(recEscalation, reqEscalation)
	assert.Equal(t, http.StatusBadRequest, recEscalation.Code)
	assert.Contains(t, recEscalation.Body.String(), "E-11")

	// 3. Registro exitoso de comensal (rol CUSTOMER por defecto) -> 201 Created
	bodyValid, _ := json.Marshal(models.UserRegisterRequest{
		Email:    "comensal@test.com",
		Password: "Password123",
		ClientID: "reservas-web",
	})
	reqValid := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(bodyValid))
	reqValid.Header.Set("Content-Type", "application/json")
	recValid := httptest.NewRecorder()
	router.ServeHTTP(recValid, reqValid)

	assert.Equal(t, http.StatusCreated, recValid.Code)
	var tokenResp models.TokenResponse
	err := json.Unmarshal(recValid.Body.Bytes(), &tokenResp)
	require.NoError(t, err)
	assert.Equal(t, "comensal@test.com", tokenResp.User)
	assert.Equal(t, "CUSTOMER", tokenResp.Role)
	assert.Equal(t, "reservas-web", tokenResp.ClientID)
	assert.NotEmpty(t, tokenResp.Token)

	// 4. Registro duplicado con el mismo correo -> 409 Conflict [E-12]
	reqDuplicate := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(bodyValid))
	reqDuplicate.Header.Set("Content-Type", "application/json")
	recDuplicate := httptest.NewRecorder()
	router.ServeHTTP(recDuplicate, reqDuplicate)
	assert.Equal(t, http.StatusConflict, recDuplicate.Code)
	assert.Contains(t, recDuplicate.Body.String(), "E-12")
}

func TestFlexibleRolesAndAdmin(t *testing.T) {
	router, _, cfg, backend := setupTestEnvironment(t)
	defer backend.Close()

	// Token de admin válido
	adminToken, err := security.GenerateToken("admin@test.com", "ADMIN", "admin-client", cfg.SecretKey, 15*time.Minute)
	require.NoError(t, err)

	// 1. Admin crea usuario con rol STAFF
	staffReq, _ := json.Marshal(models.AdminUserCreateRequest{
		Email:    "staff1@restaurant.com",
		Password: "StaffPassword1",
		Role:     "STAFF",
	})
	reqStaff := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(staffReq))
	reqStaff.Header.Set("Content-Type", "application/json")
	reqStaff.Header.Set("Authorization", "Bearer "+adminToken)
	recStaff := httptest.NewRecorder()
	router.ServeHTTP(recStaff, reqStaff)
	assert.Equal(t, http.StatusCreated, recStaff.Code)
	assert.Contains(t, recStaff.Body.String(), "STAFF")

	// 2. Admin crea usuario con rol SUPERADMIN
	superAdminReq, _ := json.Marshal(models.AdminUserCreateRequest{
		Email:    "superadmin@platform.com",
		Password: "SuperPassword1",
		Role:     "SUPERADMIN",
	})
	reqSuperAdmin := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(superAdminReq))
	reqSuperAdmin.Header.Set("Content-Type", "application/json")
	reqSuperAdmin.Header.Set("Authorization", "Bearer "+adminToken)
	recSuperAdmin := httptest.NewRecorder()
	router.ServeHTTP(recSuperAdmin, reqSuperAdmin)
	assert.Equal(t, http.StatusCreated, recSuperAdmin.Code)
	assert.Contains(t, recSuperAdmin.Body.String(), "SUPERADMIN")

	// 3. Usuario con rol SUPERADMIN puede acceder a endpoints administrativos
	superAdminToken, err := security.GenerateToken("superadmin@platform.com", "SUPERADMIN", "admin-client", cfg.SecretKey, 15*time.Minute)
	require.NoError(t, err)

	userBySuperReq, _ := json.Marshal(models.AdminUserCreateRequest{
		Email:    "customer1@test.com",
		Password: "CustomerPass1",
		Role:     "CUSTOMER",
	})
	reqUserBySuper := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(userBySuperReq))
	reqUserBySuper.Header.Set("Content-Type", "application/json")
	reqUserBySuper.Header.Set("Authorization", "Bearer "+superAdminToken)
	recUserBySuper := httptest.NewRecorder()
	router.ServeHTTP(recUserBySuper, reqUserBySuper)
	assert.Equal(t, http.StatusCreated, recUserBySuper.Code)
}

func TestPublicPathsProxying(t *testing.T) {
	router, mockDB, cfg, backend := setupTestEnvironment(t)
	defer backend.Close()

	// Sembrar comensal de prueba en mockDB para resolución de identidad
	mockDB.users["comensal@test.com"] = &models.User{
		ID:    "guest-uuid-123",
		Email: "comensal@test.com",
		Role:  "CUSTOMER",
	}

	// 1. Acceso anónimo a sub-ruta pública (/catalog/items) sin token -> 200 OK
	reqPublic := httptest.NewRequest(http.MethodGet, "/catalog/items", nil)
	reqPublic.Header.Set("X-Client-Id", "reservas-web")
	recPublic := httptest.NewRecorder()
	router.ServeHTTP(recPublic, reqPublic)

	assert.Equal(t, http.StatusOK, recPublic.Code)
	assert.Contains(t, recPublic.Body.String(), "Item A")

	headers := getLastProxyHeaders()
	require.NotNil(t, headers)
	assert.Equal(t, "catalog-secret-api-key", headers.Get("X-Internal-Api-Key"))
	assert.Equal(t, "reservas-web", headers.Get("X-Client-Id"))
	assert.Empty(t, headers.Get("X-User-ID"))

	// 2. Acceso anónimo a sub-ruta no pública (/catalog/orders) sin token -> 401 Unauthorized [E-08]
	reqPrivateNoToken := httptest.NewRequest(http.MethodGet, "/catalog/orders", nil)
	recPrivateNoToken := httptest.NewRecorder()
	router.ServeHTTP(recPrivateNoToken, reqPrivateNoToken)

	assert.Equal(t, http.StatusUnauthorized, recPrivateNoToken.Code)
	assert.Contains(t, recPrivateNoToken.Body.String(), "E-08")

	// 3. Acceso a sub-ruta pública pero CON token válido de comensal -> 200 OK y propaga X-User-ID
	guestToken, err := security.GenerateToken("comensal@test.com", "CUSTOMER", "reservas-web", cfg.SecretKey, 15*time.Minute)
	require.NoError(t, err)

	reqPublicWithToken := httptest.NewRequest(http.MethodGet, "/catalog/items", nil)
	reqPublicWithToken.Header.Set("Authorization", "Bearer "+guestToken)
	reqPublicWithToken.Header.Set("X-Client-Id", "reservas-web")
	recPublicWithToken := httptest.NewRecorder()
	router.ServeHTTP(recPublicWithToken, reqPublicWithToken)

	assert.Equal(t, http.StatusOK, recPublicWithToken.Code)
	headersWithUser := getLastProxyHeaders()
	require.NotNil(t, headersWithUser)
	assert.Equal(t, "comensal@test.com", headersWithUser.Get("X-User-Email"))
	assert.Equal(t, "CUSTOMER", headersWithUser.Get("X-User-Role"))
	assert.Equal(t, "reservas-web", headersWithUser.Get("X-Client-Id"))
	assert.Equal(t, "guest-uuid-123", headersWithUser.Get("X-User-ID"))
}



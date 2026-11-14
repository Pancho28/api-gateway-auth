package auth

import (
	"context"
	"sync"
	"testing"

	"api-gateway/internal/config"
	"api-gateway/internal/models"
	"api-gateway/internal/redis"
	"api-gateway/internal/security"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserRepo struct {
	mu    sync.RWMutex
	users map[string]*models.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[string]*models.User),
	}
}

func (m *mockUserRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserRepo) CreateUser(ctx context.Context, user *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.Email] = user
	return nil
}

func (m *mockUserRepo) DeleteUser(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for email, u := range m.users {
		if u.ID == id {
			delete(m.users, email)
			return nil
		}
	}
	return nil
}

func TestServiceLogin(t *testing.T) {
	cfg := &config.Config{
		SecretKey:                "test-secret-key-1234567890123456",
		AccessTokenExpireMinutes: 60,
	}
	repo := newMockUserRepo()
	rdb := redis.NewInMemoryClient()

	svc := NewService(repo, rdb, cfg)

	hashedPassword, err := security.HashPassword("Password123")
	require.NoError(t, err)

	expectedUserID := uuid.New().String()
	userEmail := "loginuser@test.com"
	repo.users[userEmail] = &models.User{
		ID:             expectedUserID,
		Email:          userEmail,
		HashedPassword: hashedPassword,
		Role:           models.RoleUser,
	}

	// 1. Login exitoso -> Debe retornar res.ID con UUIDv4 válido y mapeado en JWT claims.ID
	res, err := svc.Login(context.Background(), &models.UserLoginRequest{
		Email:    userEmail,
		Password: "Password123",
		ClientID: "client-test",
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, expectedUserID, res.ID, "res.ID debe coincidir exactamente con el UUID del usuario")
	_, errUUID := uuid.Parse(res.ID)
	assert.NoError(t, errUUID, "res.ID debe ser un UUIDv4 sintácticamente válido")

	assert.Equal(t, userEmail, res.User)
	assert.Equal(t, models.RoleUser, res.Role)
	assert.Equal(t, "client-test", res.ClientID)
	assert.NotEmpty(t, res.Token)

	// Validar que el JWT firmado contenga res.ID en sus claims
	claims, errClaims := security.ValidateToken(res.Token, cfg.SecretKey)
	require.NoError(t, errClaims)
	assert.Equal(t, res.ID, claims.ID, "claims.ID debe ser idéntico al ID del usuario")
	assert.Equal(t, userEmail, claims.Subject)

	// 2. Contraseña incorrecta -> ErrInvalidCredentials
	_, errBadPass := svc.Login(context.Background(), &models.UserLoginRequest{
		Email:    userEmail,
		Password: "WrongPassword",
		ClientID: "client-test",
	})
	assert.ErrorIs(t, errBadPass, ErrInvalidCredentials)

	// 3. Usuario inexistente -> ErrInvalidCredentials
	_, errNotFound := svc.Login(context.Background(), &models.UserLoginRequest{
		Email:    "nonexistent@test.com",
		Password: "Password123",
		ClientID: "client-test",
	})
	assert.ErrorIs(t, errNotFound, ErrInvalidCredentials)
}

func TestServiceRegister(t *testing.T) {
	cfg := &config.Config{
		SecretKey:                "test-secret-key-1234567890123456",
		AccessTokenExpireMinutes: 60,
	}
	repo := newMockUserRepo()
	rdb := redis.NewInMemoryClient()

	svc := NewService(repo, rdb, cfg)

	// 1. Registro sin ID previo -> El Gateway genera un UUIDv4
	res, err := svc.Register(context.Background(), &models.UserRegisterRequest{
		Email:    "newguest@test.com",
		Password: "Password123",
		ClientID: "client-reg",
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.NotEmpty(t, res.ID, "El ID no debe estar vacío")
	_, errUUID := uuid.Parse(res.ID)
	assert.NoError(t, errUUID, "El ID generado debe ser un UUIDv4 válido")
	assert.Equal(t, "newguest@test.com", res.User)
	assert.Equal(t, models.RoleUser, res.Role)
	assert.Equal(t, "client-reg", res.ClientID)
	assert.NotEmpty(t, res.Token)

	// Validar claim de JWT
	claims, errClaims := security.ValidateToken(res.Token, cfg.SecretKey)
	require.NoError(t, errClaims)
	assert.Equal(t, res.ID, claims.ID)
	assert.Equal(t, "newguest@test.com", claims.Subject)

	// 2. Registro con Client-Generated UUID
	customID := "b3e94472-132d-425d-bb57-9db078652f10"
	resCustom, err := svc.Register(context.Background(), &models.UserRegisterRequest{
		ID:       customID,
		Email:    "customid@test.com",
		Password: "Password123",
		ClientID: "client-reg",
	})
	require.NoError(t, err)
	require.NotNil(t, resCustom)
	assert.Equal(t, customID, resCustom.ID, "Debe preservar el UUID suministrado por el cliente")

	claimsCustom, errClaimsCustom := security.ValidateToken(resCustom.Token, cfg.SecretKey)
	require.NoError(t, errClaimsCustom)
	assert.Equal(t, customID, claimsCustom.ID)

	// 3. Intento de registro con email duplicado -> ErrEmailAlreadyExists
	_, errDup := svc.Register(context.Background(), &models.UserRegisterRequest{
		Email:    "newguest@test.com",
		Password: "Password123",
		ClientID: "client-reg",
	})
	assert.ErrorIs(t, errDup, ErrEmailAlreadyExists)
}

func TestServiceLogout(t *testing.T) {
	cfg := &config.Config{
		SecretKey:                "test-secret-key-1234567890123456",
		AccessTokenExpireMinutes: 60,
	}
	repo := newMockUserRepo()
	rdb := redis.NewInMemoryClient()

	svc := NewService(repo, rdb, cfg)

	hashedPassword, err := security.HashPassword("Password123")
	require.NoError(t, err)

	userEmail := "logoutuser@test.com"
	repo.users[userEmail] = &models.User{
		ID:             uuid.New().String(),
		Email:          userEmail,
		HashedPassword: hashedPassword,
		Role:           models.RoleUser,
	}

	// 1. Iniciar sesión para poblar Redis con la sesión
	loginRes, err := svc.Login(context.Background(), &models.UserLoginRequest{
		Email:    userEmail,
		Password: "Password123",
		ClientID: "client-logout",
	})
	require.NoError(t, err)
	require.NotNil(t, loginRes)

	sessionKey := "session:" + userEmail + ":client-logout"
	var sessionData models.SessionData
	existsInRedis := rdb.GetJSON(context.Background(), sessionKey, &sessionData)
	assert.True(t, existsInRedis, "La sesión particionada por client_id debe existir en Redis tras el login")
	assert.Equal(t, userEmail, sessionData.Email)
	assert.Equal(t, "client-logout", sessionData.ClientID)

	claims, errClaims := security.ValidateToken(loginRes.Token, cfg.SecretKey)
	require.NoError(t, errClaims)
	require.NotEmpty(t, claims.RegisteredClaims.ID)

	// 2. Ejecutar Logout exitoso -> Elimina la clave de sesión y registra jti en denylist
	errLogout := svc.Logout(context.Background(), loginRes.Token, "client-logout")
	assert.NoError(t, errLogout)

	var sessionAfter models.SessionData
	existsAfter := rdb.GetJSON(context.Background(), sessionKey, &sessionAfter)
	assert.False(t, existsAfter, "La sesión ya no debe existir en Redis tras el logout")
	assert.True(t, rdb.Exists(context.Background(), "revoked_jwt:"+claims.RegisteredClaims.ID), "El jti debe quedar registrado en la denylist de Redis tras el logout")

	// 3. Idempotencia: Logout cuando la sesión ya no está en Redis -> Debe responder sin error
	errLogoutAgain := svc.Logout(context.Background(), loginRes.Token, "client-logout")
	assert.NoError(t, errLogoutAgain, "Logout debe ser idempotente si la sesión no está en Redis")

	// 4. Sesiones simultáneas en múltiples clientes no se sobrescriben
	loginClientB, err := svc.Login(context.Background(), &models.UserLoginRequest{
		Email:    userEmail,
		Password: "Password123",
		ClientID: "client-mobile",
	})
	require.NoError(t, err)
	assert.True(t, rdb.Exists(context.Background(), "session:"+userEmail+":client-mobile"))
	assert.NotEmpty(t, loginClientB.Token)

	// 5. Client-ID mismatch (Token Binding) -> ErrTokenBindingMismatch
	errMismatch := svc.Logout(context.Background(), loginRes.Token, "different-client")
	assert.ErrorIs(t, errMismatch, ErrTokenBindingMismatch)

	// 6. Token inválido o malformado -> ErrInvalidCredentials
	errBadToken := svc.Logout(context.Background(), "invalid-token-string", "client-logout")
	assert.ErrorIs(t, errBadToken, ErrInvalidCredentials)
}

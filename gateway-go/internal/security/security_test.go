package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordHashing(t *testing.T) {
	password := "SecretPass123"

	hash, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Verificación exitosa
	assert.True(t, VerifyPassword(password, hash))

	// Contraseña errónea
	assert.False(t, VerifyPassword("WrongPass", hash))

	// Compatibilidad hacia atrás con hash generado por Passlib en Python
	pythonHash := "$pbkdf2-sha256$29000$uLeWUopRyrn33hsj5Nw7Jw$GWqt0P3bepkAma1YXol/g2/ekQMnNR70rKhtSb7FHls"
	assert.True(t, VerifyPassword("Admin1234", pythonHash))
	assert.False(t, VerifyPassword("Incorrect", pythonHash))
}

func TestJWTTokenLifecycle(t *testing.T) {
	secret := "super-secret-key-for-testing-purposes-123"
	email := "user@example.com"
	role := "USER"

	// 1. Generar token
	token, err := GenerateToken(email, role, secret, 30*time.Minute)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// 2. Decodificar y validar
	claims, err := ValidateToken(token, secret)
	require.NoError(t, err)
	assert.Equal(t, email, claims.Subject)
	assert.Equal(t, role, claims.Role)

	// 3. Clave secreta incorrecta
	_, err = ValidateToken(token, "wrong-key")
	assert.ErrorIs(t, err, ErrInvalidToken)

	// 4. Token expirado
	expiredToken, err := GenerateToken(email, role, secret, -5*time.Minute)
	require.NoError(t, err)
	_, err = ValidateToken(expiredToken, secret)
	assert.ErrorIs(t, err, ErrInvalidToken)

	// 5. Token corrupto
	_, err = ValidateToken("invalid.corrupt.token", secret)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

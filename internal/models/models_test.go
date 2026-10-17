package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdminUserCreateValidation(t *testing.T) {
	// 1. Caso Válido
	req := AdminUserCreateRequest{
		Email:    "newuser@test.com",
		Password: "Password123",
		Role:     "USER",
	}
	assert.NoError(t, req.Validate())

	// 2. Contraseña demasiado corta (< 8 caracteres)
	reqShortPass := req
	reqShortPass.Password = "Pass1"
	assert.Error(t, reqShortPass.Validate())

	// 3. Contraseña demasiado larga (> 40 caracteres)
	reqLongPass := req
	reqLongPass.Password = strings.Repeat("A1", 21) // 42 caracteres
	assert.Error(t, reqLongPass.Validate())

	// 4. Complejidad de contraseña: Solo letras (sin dígitos)
	reqNoDigits := req
	reqNoDigits.Password = "PasswordOnlyLetters"
	assert.Error(t, reqNoDigits.Validate())

	// 5. Complejidad de contraseña: Solo números (sin letras)
	reqNoLetters := req
	reqNoLetters.Password = "123456789012"
	assert.Error(t, reqNoLetters.Validate())

	// 6. Email demasiado largo (> 50 caracteres)
	reqLongEmail := req
	reqLongEmail.Email = "verylongemailaddressexceedingfiftycharacterslimit@domain.com"
	assert.Error(t, reqLongEmail.Validate())

	// 7. Rol inválido
	reqInvalidRole := req
	reqInvalidRole.Role = "SUPERADMIN"
	assert.Error(t, reqInvalidRole.Validate())

	// 8. Normalización de rol en minúsculas a mayúsculas
	reqLowerRole := req
	reqLowerRole.Role = "admin"
	assert.NoError(t, reqLowerRole.Validate())
	assert.Equal(t, "ADMIN", reqLowerRole.Role)
}

func TestUserLoginValidation(t *testing.T) {
	// 1. Caso Válido
	validLogin := UserLoginRequest{
		Email:    "user@domain.com",
		Password: "ValidPassword1",
	}
	assert.NoError(t, validLogin.Validate())

	// 2. Email inválido
	invalidEmail := validLogin
	invalidEmail.Email = "invalid-email-format"
	assert.Error(t, invalidEmail.Validate())

	// 3. Contraseña vacía
	emptyPass := validLogin
	emptyPass.Password = ""
	assert.Error(t, emptyPass.Validate())

	// 4. Contraseña que excede 40 caracteres
	longPass := validLogin
	longPass.Password = strings.Repeat("X", 41)
	assert.Error(t, longPass.Validate())

	// 5. ClientID válido
	validClient := validLogin
	validClient.ClientID = "app-comensales_v1"
	assert.NoError(t, validClient.Validate())

	// 6. ClientID con caracteres inválidos (espacios o símbolos raros)
	invalidClient := validLogin
	invalidClient.ClientID = "app with spaces!"
	assert.Error(t, invalidClient.Validate())

	// 7. ClientID demasiado largo (> 50 chars)
	longClient := validLogin
	longClient.ClientID = strings.Repeat("a", 51)
	assert.Error(t, longClient.Validate())
}

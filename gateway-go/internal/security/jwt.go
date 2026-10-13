package security

import (
	"errors"
	"fmt"
	"time"

	"api-gateway/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("token inválido o expirado")
	ErrMissingEmail = errors.New("el token no contiene un identificador de usuario válido")
)

// GenerateToken crea un token JWT firmado con HS256 conteniendo el email y el rol
func GenerateToken(email, role, secretKey string, expireDuration time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := models.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expireDuration)),
		},
		Role: role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("error al firmar token JWT: %w", err)
	}

	return tokenString, nil
}

// ValidateToken decodifica y verifica la firma y expiración del token JWT
func ValidateToken(tokenString, secretKey string) (*models.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de firma no esperado: %v", t.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*models.JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.Subject == "" {
		return nil, ErrMissingEmail
	}

	return claims, nil
}

package security

import (
	"errors"
	"fmt"
	"time"

	"api-gateway/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken  = errors.New("token inválido o expirado")
	ErrMissingEmail  = errors.New("el token no contiene un identificador de usuario válido")
	ErrMissingClient = errors.New("el token no contiene un identificador de cliente válido")
)

// TokenOptions define los parámetros para la emisión de un token JWT
type TokenOptions struct {
	Email          string
	Role           string
	ClientID       string
	Audiences      []string
	Scopes         []string
	SecretKey      string
	ExpireDuration time.Duration
}

// GenerateTokenWithOptions crea un token JWT firmado con HS256 con opciones extendidas de audiencia y particionamiento
func GenerateTokenWithOptions(opts TokenOptions) (string, error) {
	now := time.Now().UTC()
	var aud jwt.ClaimStrings
	if len(opts.Audiences) > 0 {
		aud = opts.Audiences
	} else if opts.ClientID != "" {
		aud = jwt.ClaimStrings{opts.ClientID}
	}

	claims := models.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   opts.Email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(opts.ExpireDuration)),
			Audience:  aud,
		},
		Role:     opts.Role,
		ClientID: opts.ClientID,
		Scopes:   opts.Scopes,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(opts.SecretKey))
	if err != nil {
		return "", fmt.Errorf("error al firmar token JWT: %w", err)
	}

	return tokenString, nil
}

// GenerateToken crea un token JWT firmado con HS256 conteniendo el email, el rol y el client_id
func GenerateToken(email, role, clientID, secretKey string, expireDuration time.Duration) (string, error) {
	return GenerateTokenWithOptions(TokenOptions{
		Email:          email,
		Role:           role,
		ClientID:       clientID,
		SecretKey:      secretKey,
		ExpireDuration: expireDuration,
	})
}

// ValidateToken decodifica y verifica la firma, expiración e identidad de cliente del token JWT
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

	if claims.ClientID == "" {
		return nil, ErrMissingClient
	}

	return claims, nil
}

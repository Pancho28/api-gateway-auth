package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	DefaultRounds = 29000
	SaltLength    = 16
	KeyLength     = 32
)

var b64Encoding = base64.RawStdEncoding

// HashPassword genera un hash seguro compatible con el formato pbkdf2_sha256 de Passlib
func HashPassword(password string) (string, error) {
	salt := make([]byte, SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("error al generar salt aleatorio: %w", err)
	}

	dk := pbkdf2.Key([]byte(password), salt, DefaultRounds, KeyLength, sha256.New)

	saltB64 := b64Encoding.EncodeToString(salt)
	digestB64 := b64Encoding.EncodeToString(dk)

	return fmt.Sprintf("$pbkdf2-sha256$%d$%s$%s", DefaultRounds, saltB64, digestB64), nil
}

// VerifyPassword valida si una contraseña en texto plano coincide con el hash en formato Passlib
func VerifyPassword(plainPassword, hashedPassword string) bool {
	parts := strings.Split(hashedPassword, "$")
	if len(parts) != 5 || parts[1] != "pbkdf2-sha256" {
		return false
	}

	rounds, err := strconv.Atoi(parts[2])
	if err != nil || rounds <= 0 {
		return false
	}

	salt, err := decodeBase64Safe(parts[3])
	if err != nil {
		return false
	}

	expectedChecksum, err := decodeBase64Safe(parts[4])
	if err != nil {
		return false
	}

	derivedKey := pbkdf2.Key([]byte(plainPassword), salt, rounds, len(expectedChecksum), sha256.New)

	return subtle.ConstantTimeCompare(derivedKey, expectedChecksum) == 1
}

func decodeBase64Safe(s string) ([]byte, error) {
	if data, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	return nil, errors.New("formato base64 inválido")
}

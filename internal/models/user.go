package models

import (
	"errors"
	"regexp"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	hasLetterRegex = regexp.MustCompile(`[a-zA-Z]`)
	hasDigitRegex  = regexp.MustCompile(`[0-9]`)
	clientRegex   = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// User representa el modelo de entidad en la base de datos y en el contexto de autenticación
type User struct {
	ID             string   `json:"id"`
	Email          string   `json:"email"`
	HashedPassword string   `json:"-"`
	Role           string   `json:"role"`
	ClientID       string   `json:"client_id,omitempty"`
	Scopes         []string `json:"scopes,omitempty"`
}

// UserLoginRequest representa el cuerpo de la petición para /auth/login
type UserLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	ClientID string `json:"client_id,omitempty"`
}

func (r *UserLoginRequest) Validate() error {
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" || len(r.Email) > 50 {
		return errors.New("el correo electrónico es inválido o excede los 50 caracteres")
	}
	if !emailRegex.MatchString(r.Email) {
		return errors.New("formato de correo electrónico inválido")
	}
	if r.Password == "" || len(r.Password) > 40 {
		return errors.New("la contraseña es inválida o excede los 40 caracteres")
	}
	r.ClientID = strings.TrimSpace(r.ClientID)
	if r.ClientID != "" {
		if len(r.ClientID) > 50 || !clientRegex.MatchString(r.ClientID) {
			return errors.New("el identificador de cliente es inválido (máx 50 caracteres alfanuméricos, guiones o guiones bajos)")
		}
	}
	return nil
}

// TokenResponse representa la respuesta entregada tras un login exitoso
type TokenResponse struct {
	User     string `json:"user"`
	Role     string `json:"role"`
	Token    string `json:"token"`
	ClientID string `json:"client_id,omitempty"`
}

// AdminUserCreateRequest representa la petición de creación de usuario por un ADMIN
type AdminUserCreateRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (r *AdminUserCreateRequest) Validate() error {
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" || len(r.Email) > 50 {
		return errors.New("el correo electrónico es obligatorio y no debe exceder 50 caracteres")
	}
	if !emailRegex.MatchString(r.Email) {
		return errors.New("formato de correo electrónico inválido")
	}

	if len(r.Password) < 8 || len(r.Password) > 40 {
		return errors.New("la contraseña debe tener entre 8 y 40 caracteres")
	}
	if !hasLetterRegex.MatchString(r.Password) || !hasDigitRegex.MatchString(r.Password) {
		return errors.New("la contraseña debe contener al menos una letra y un número")
	}

	r.Role = strings.ToUpper(strings.TrimSpace(r.Role))
	if r.Role != "ADMIN" && r.Role != "USER" {
		return errors.New("el rol debe ser ADMIN o USER")
	}
	return nil
}

// UserResponse representa la respuesta devuelta al crear un nuevo usuario
type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// JWTClaims representa la información decodificada del token con soporte de audiencia y client_id
type JWTClaims struct {
	jwt.RegisteredClaims
	Role     string   `json:"role"`
	ClientID string   `json:"client_id"`
	Scopes   []string `json:"scopes,omitempty"`
}

// SessionData representa los datos de sesión almacenados en Redis
type SessionData struct {
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	Role     string   `json:"role"`
	ClientID string   `json:"client_id"`
	Scopes   []string `json:"scopes,omitempty"`
}
